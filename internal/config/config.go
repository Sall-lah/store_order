package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config aggregates all operational parameters required to boot and run the store_order microservice.
type Config struct {
	Port                  string
	GRPCPort              string
	Dev                   bool
	EnableDocs            bool
	DatabaseURL           string
	KafkaBrokers          []string
	KafkaTopicUserEvents  string
	KafkaUserEventsGroupID string
	ProductServiceURL     string
	MidtransServerKey     string
	MidtransClientKey     string
	MidtransIsProduction bool
	RedisURL              string
	RedisPassword         string
	RedisRateLimitEnabled bool
}

// Load loads environment variables from .env if present and constructs a validated Config instance.
// Why: Centralizes environment parsing and validation at application bootstrap to ensure fast-fail configuration integrity.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[Config] No .env file found or failed to load, falling back to system environment variables.")
	}

	port := getEnv("PORT", "8060")
	grpcPort := getEnv("GRPC_PORT", "50051")
	devStr := strings.ToLower(strings.TrimSpace(getEnv("DEV", "false")))
	dev := devStr == "true" || devStr == "1"

	enableDocsStr := strings.ToLower(strings.TrimSpace(getEnv("ENABLE_DOCS", "true")))
	enableDocs := enableDocsStr == "true" || enableDocsStr == "1"

	databaseURL := getEnv("DATABASE_URL", "")
	
	kafkaBrokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	kafkaBrokers := splitAndTrim(kafkaBrokersStr, ",")
	kafkaTopicUserEvents := getEnv("KAFKA_TOPIC_USER_EVENTS", "user.events")
	kafkaUserEventsGroupID := getEnv("KAFKA_USER_EVENTS_GROUP_ID", "store_order_user_events")

	productServiceURL := strings.TrimRight(getEnv("PRODUCT_SERVICE_URL", "http://localhost:8040"), "/")

	midtransServerKey := getEnv("MIDTRANS_SERVER_KEY", "")
	midtransClientKey := getEnv("MIDTRANS_CLIENT_KEY", "")
	midtransIsProd, _ := strconv.ParseBool(getEnv("MIDTRANS_IS_PRODUCTION", "false"))

	redisHost := getEnv("REDIS_HOST", "")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisURL := getEnv("REDIS_URL", "localhost:6379")
	if redisHost != "" {
		redisURL = fmt.Sprintf("%s:%s", redisHost, redisPort)
	}
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisRateLimitEnabledStr := strings.ToLower(strings.TrimSpace(getEnv("REDIS_RATE_LIMIT_ENABLED", "true")))
	redisRateLimitEnabled := redisRateLimitEnabledStr == "true" || redisRateLimitEnabledStr == "1"

	return &Config{
		Port:                  port,
		GRPCPort:              grpcPort,
		Dev:                   dev,
		EnableDocs:            enableDocs,
		DatabaseURL:           databaseURL,
		KafkaBrokers:          kafkaBrokers,
		KafkaTopicUserEvents:  kafkaTopicUserEvents,
		KafkaUserEventsGroupID: kafkaUserEventsGroupID,
		ProductServiceURL:     productServiceURL,
		MidtransServerKey:     midtransServerKey,
		MidtransClientKey:     midtransClientKey,
		MidtransIsProduction:  midtransIsProd,
		RedisURL:              redisURL,
		RedisPassword:         redisPassword,
		RedisRateLimitEnabled: redisRateLimitEnabled,
	}
}

// getEnv retrieves an environment variable or returns the default value if unset.
func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && strings.TrimSpace(val) != "" {
		return val
	}
	return defaultVal
}

// splitAndTrim splits a delimited string and trims whitespace around each element.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var res []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	if len(res) == 0 {
		return []string{"localhost:9092"}
	}
	return res
}
