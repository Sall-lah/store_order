package router

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Sall-lah/store_order/internal/config"
	"github.com/Sall-lah/store_order/internal/handler"
	"github.com/Sall-lah/store_order/internal/middleware"
	"github.com/Sall-lah/store_order/internal/ratelimit"
)

// RouterDeps holds handler dependencies required to configure the HTTP router.
type RouterDeps struct {
	Config         *config.Config
	OrderHandler   *handler.OrderHandler
	WebhookHandler *handler.WebhookHandler
	AdminHandler   *handler.AdminHandler
	DevHandler     *handler.DevHandler
	HealthHandler  *handler.HealthHandler
	RateLimiter    ratelimit.Limiter
}

// SetupRouter initializes and configures the Chi HTTP multiplexer with routing rules and middleware.
// Why: Centralizes HTTP endpoint registration, security boundaries, and documentation proxies.
func SetupRouter(deps RouterDeps) *chi.Mux {
	r := chi.NewRouter()

	// 1. Base Middlewares
	r.Use(middleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(middleware.Recoverer)

	// Global Volumetric IP Rate Limiting (120 req/min per client IP)
	if deps.RateLimiter != nil {
		r.Use(middleware.RequireRateLimit(deps.RateLimiter, middleware.RateLimitRule{
			Limit:  120,
			Window: time.Minute,
			Keyer:  middleware.KeyByIP,
		}))
	}

	// 2. CORS configuration (Permissive for gateway / local dev)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID", "X-User-Id", "X-User-Role", "X-User-Email"},
		ExposedHeaders:   []string{"Link", "X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 3. User Identity Extraction from Gateway Headers
	r.Use(middleware.ExtractGatewayUser)

	// 4. Health Check Probe
	r.Get("/health", deps.HealthHandler.Check)

	// 5. Documentation Endpoints
	if deps.Config.EnableDocs {
		registerDocumentationRoutes(r)
	}

	// 6. Public Webhook & Protected Customer Endpoints
	r.Route("/api/v1/orders", func(r chi.Router) {
		// Public Webhook from Midtrans (Scoped per source IP: 300 req/min)
		webhookRouter := r
		if deps.RateLimiter != nil {
			webhookRouter = r.With(middleware.RequireRateLimit(deps.RateLimiter, middleware.RateLimitRule{
				Limit:  300,
				Window: time.Minute,
				Keyer:  middleware.KeyWithPrefix("webhook", middleware.KeyByIP),
			}))
		}
		webhookRouter.Post("/webhook/midtrans", deps.WebhookHandler.HandleMidtrans)

		// Protected Customer Order Operations
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)

			// Checkout Rate Limiting: 3 requests per 10s per User ID
			checkoutRouter := r
			if deps.RateLimiter != nil {
				checkoutRouter = r.With(middleware.RequireRateLimit(deps.RateLimiter, middleware.RateLimitRule{
					Limit:  3,
					Window: 10 * time.Second,
					Keyer:  middleware.KeyWithPrefix("checkout", middleware.KeyByUser),
				}))
			}
			checkoutRouter.Post("/", deps.OrderHandler.Checkout)

			r.Get("/", deps.OrderHandler.ListOrders)
			r.Get("/{id}", deps.OrderHandler.GetOrder)

			// Cancellation Rate Limiting: 5 requests per minute per User ID
			cancelRouter := r
			if deps.RateLimiter != nil {
				cancelRouter = r.With(middleware.RequireRateLimit(deps.RateLimiter, middleware.RateLimitRule{
					Limit:  5,
					Window: time.Minute,
					Keyer:  middleware.KeyWithPrefix("cancel", middleware.KeyByUser),
				}))
			}
			cancelRouter.Post("/{id}/cancel", deps.OrderHandler.CancelOrder)
		})
	})

	// 7. Admin Order Operations (Guarded by RequireAdmin)
	r.Route("/api/v1/admin/orders", func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Get("/", deps.AdminHandler.ListOrders)
		r.Patch("/{id}/status", deps.AdminHandler.UpdateOrderStatus)
	})

	// 8. Developer Simulation Endpoints (Only enabled when DEV="TRUE")
	if deps.Config.Dev {
		r.Route("/api/v1/dev/orders", func(r chi.Router) {
			r.Post("/{id}/simulate-success", deps.DevHandler.SimulateSuccess)
			r.Post("/{id}/simulate-cancel", deps.DevHandler.SimulateCancel)
			r.Post("/{id}/simulate-expire", deps.DevHandler.SimulateExpire)
		})
	}

	return r
}

func registerDocumentationRoutes(r *chi.Mux) {
	// Raw OpenAPI Specs
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		serveFileIfExists(w, r, "docs/openapi.yaml")
	})

	r.Get("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		serveFileIfExists(w, r, "docs/openapi.json")
	})

	// Modern Scalar UI
	// Why: Normalizing trailing slashes ensures browser relative path resolution deterministically targets ./openapi.json
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
	})

	r.Get("/docs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!doctype html>
<html>
  <head>
    <title>Store Order API Documentation</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script id="api-reference" data-url="./openapi.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})

	// Classic Swagger UI
	// Why: Trailing slash redirect ensures relative '../docs/openapi.json' resolves against the common base path
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
	})

	r.Get("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI - Store Order</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "../docs/openapi.json",
        dom_id: '#swagger-ui',
        deepLinking: true
      });
    };
  </script>
</body>
</html>`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	})
}

func serveFileIfExists(w http.ResponseWriter, r *http.Request, relPath string) {
	candidates := []string{
		relPath,
		filepath.Join(".", relPath),
		filepath.Join("..", relPath),
		filepath.Join("..", "..", relPath),
		filepath.Join("/app", relPath),
	}

	for _, path := range candidates {
		if content, err := os.ReadFile(path); err == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}
	}

	http.Error(w, fmt.Sprintf("Specification file %s not found.", relPath), http.StatusNotFound)
}
