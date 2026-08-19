package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sall-lah/store_order/internal/middleware"
)

func TestExtractGatewayUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-User-Id", "usr_test_123")
	req.Header.Set("X-User-Email", "test@example.com")
	req.Header.Set("X-User-Role", "customer")

	handler := middleware.ExtractGatewayUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetUserFromContext(r.Context())
		if !ok {
			t.Fatal("expected user in context")
		}
		if user.ID != "usr_test_123" {
			t.Errorf("expected user ID usr_test_123, got %s", user.ID)
		}
		if user.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", user.Email)
		}
		if user.Role != "customer" {
			t.Errorf("expected role customer, got %s", user.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestRequireAuth(t *testing.T) {
	handler := middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Without auth
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. With auth context
	ctx := context.WithValue(req.Context(), middleware.UserCtxKey, middleware.GatewayUser{ID: "usr_123"})
	reqWithCtx := req.WithContext(ctx)
	recWithCtx := httptest.NewRecorder()
	handler.ServeHTTP(recWithCtx, reqWithCtx)
	if recWithCtx.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", recWithCtx.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	handler := middleware.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Customer role
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	ctx := context.WithValue(req.Context(), middleware.UserCtxKey, middleware.GatewayUser{ID: "usr_1", Role: "customer"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rec.Code)
	}

	// 2. Admin role
	ctxAdmin := context.WithValue(req.Context(), middleware.UserCtxKey, middleware.GatewayUser{ID: "usr_admin", Role: "ADMIN"})
	recAdmin := httptest.NewRecorder()
	handler.ServeHTTP(recAdmin, req.WithContext(ctxAdmin))
	if recAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", recAdmin.Code)
	}
}
