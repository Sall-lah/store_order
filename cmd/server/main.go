package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Sall-lah/store_order/internal/config"
	"github.com/Sall-lah/store_order/internal/consumer"
	"github.com/Sall-lah/store_order/internal/db"
	appgrpc "github.com/Sall-lah/store_order/internal/grpc"
	"github.com/Sall-lah/store_order/internal/handler"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/integration/product"
	"github.com/Sall-lah/store_order/internal/kafka"
	"github.com/Sall-lah/store_order/internal/outbox"
	"github.com/Sall-lah/store_order/internal/ratelimit"
	"github.com/Sall-lah/store_order/internal/repository"
	"github.com/Sall-lah/store_order/internal/router"
	"github.com/Sall-lah/store_order/internal/service"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("[INFO] Starting store_order microservice...")

	// 1. Load application configuration
	cfg := config.Load()
	log.Printf("[INFO] Configuration loaded (Port: %s, DevMode: %v, ProductURL: %s)",
		cfg.Port, cfg.Dev, cfg.ProductServiceURL)

	// 2. Initialize Prisma PostgreSQL client
	prismaClient, err := db.InitClient(cfg.DatabaseURL)
	if err != nil {
		log.Printf("[WARN] Database connection warning: %v. Running in disconnected standby mode.", err)
		prismaClient = db.NewClient()
	} else {
		log.Println("[INFO] Connected successfully to PostgreSQL database.")
	}

	defer func() {
		if prismaClient != nil {
			_ = prismaClient.Prisma.Disconnect()
		}
	}()

	// 3. Initialize Kafka Producer
	kafkaProducer := kafka.NewProducer(cfg.KafkaBrokers)
	defer func() {
		_ = kafkaProducer.Close()
	}()

	// 4. Initialize Repositories
	orderRepo := repository.NewOrderRepository(prismaClient)
	outboxRepo := repository.NewOutboxRepository(prismaClient)

	// 5. Initialize Integrations
	productClient := product.NewClient(cfg.ProductServiceURL)
	midtransClient := midtrans.NewSnapClient(cfg.MidtransServerKey, cfg.MidtransIsProduction, cfg.Dev)

	// 6. Initialize Order Service, Outbox Worker & User Event Consumer
	orderService := service.NewOrderService(orderRepo, productClient, midtransClient, cfg.MidtransServerKey, cfg.Dev)
	outboxWorker := outbox.NewWorker(outboxRepo, kafkaProducer, 200*time.Millisecond, 50, cfg.Dev)
	userEventConsumer := consumer.NewUserEventConsumer(cfg.KafkaBrokers, cfg.KafkaTopicUserEvents, cfg.KafkaUserEventsGroupID, orderService, cfg.Dev)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	outboxWorker.Start(workerCtx)

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()
	userEventConsumer.Start(consumerCtx)

	// 7. Initialize Handlers
	orderHandler := handler.NewOrderHandler(orderService)
	webhookHandler := handler.NewWebhookHandler(orderService)
	adminHandler := handler.NewAdminHandler(orderService)
	devHandler := handler.NewDevHandler(orderService)
	healthHandler := handler.NewHealthHandler(prismaClient)

	// 8. Initialize Redis Rate Limiter
	var rateLimiter ratelimit.Limiter
	if cfg.RedisRateLimitEnabled {
		var redisOpt *redis.Options
		if strings.HasPrefix(cfg.RedisURL, "redis://") || strings.HasPrefix(cfg.RedisURL, "rediss://") {
			if opt, err := redis.ParseURL(cfg.RedisURL); err == nil {
				redisOpt = opt
			}
		}
		if redisOpt == nil {
			redisOpt = &redis.Options{
				Addr:     cfg.RedisURL,
				Password: cfg.RedisPassword,
			}
		}
		redisClient := redis.NewClient(redisOpt)
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := redisClient.Ping(pingCtx).Err(); err != nil {
			log.Printf("[WARN] Redis connection warning (Addr: %s): %v. Rate limiter running in fail-open mode.", cfg.RedisURL, err)
		} else {
			log.Printf("[INFO] Connected successfully to Redis at %s", cfg.RedisURL)
		}
		pingCancel()

		rateLimiter = ratelimit.NewRedisLimiter(redisClient, true, 25*time.Millisecond)
		defer func() {
			_ = rateLimiter.Close()
		}()
	} else {
		log.Println("[INFO] Redis rate limiting is disabled via configuration.")
	}

	// 9. Build Router & HTTP Server
	r := router.SetupRouter(router.RouterDeps{
		Config:         cfg,
		OrderHandler:   orderHandler,
		WebhookHandler: webhookHandler,
		AdminHandler:   adminHandler,
		DevHandler:     devHandler,
		HealthHandler:  healthHandler,
		RateLimiter:    rateLimiter,
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 10. Initialize gRPC Server
	orderGRPCService := appgrpc.NewOrderServiceServer(orderRepo)
	grpcServer := appgrpc.NewServer(cfg.GRPCPort, orderGRPCService)

	// 11. Graceful shutdown handler
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] HTTP server listening on port %s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] HTTP server runtime error: %v", err)
		}
	}()

	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("[FATAL] gRPC server runtime error: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("[INFO] Shutting down store_order service gracefully...")

	// Stop user events consumer
	consumerCancel()
	_ = userEventConsumer.Close()

	// Stop gRPC server
	grpcServer.Stop()

	// Stop outbox worker
	outboxWorker.Stop()
	workerCancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] HTTP server shutdown error: %v", err)
	}

	log.Println("[INFO] store_order service stopped gracefully.")
}
