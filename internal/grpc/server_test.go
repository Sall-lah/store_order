package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
	orderv1 "github.com/Sall-lah/store_proto/gen/go/store/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
)

const bufSize = 1024 * 1024

// setupTestGRPCServer spins up an in-memory gRPC server and returns a connected client and a cleanup function.
// Why: Provides high-speed, isolated network wire testing of gRPC RPC methods without port collisions.
func setupTestGRPCServer(t *testing.T, repo *mockOrderRepo) (orderv1.OrderServiceClient, func()) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	orderServer := NewOrderServiceServer(repo)
	orderv1.RegisterOrderServiceServer(s, orderServer)

	go func() {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("Server error: %v", err)
		}
	}()

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := orderv1.NewOrderServiceClient(conn)

	cleanup := func() {
		_ = conn.Close()
		s.GracefulStop()
		_ = lis.Close()
	}

	return client, cleanup
}

// TestGRPCClientServerIntegration performs end-to-end client-to-server RPC verification over gRPC wire transport.
func TestGRPCClientServerIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()

	repo := &mockOrderRepo{
		activeOrders: []db.OrderModel{
			{
				InnerOrder: db.InnerOrder{
					ID:          "ord_wire_1",
					OrderNumber: "ORD-WIRE-001",
					Status:      db.OrderStatusPaid,
					TotalAmount: 250000,
					CreatedAt:   now,
				},
			},
			{
				InnerOrder: db.InnerOrder{
					ID:          "ord_wire_2",
					OrderNumber: "ORD-WIRE-002",
					Status:      db.OrderStatusShipped,
					TotalAmount: 120000,
					CreatedAt:   now,
				},
			},
		},
	}

	client, cleanup := setupTestGRPCServer(t, repo)
	defer cleanup()

	formatter := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}

	t.Run("successful e2e CheckActiveOrders with active orders", func(t *testing.T) {
		resp, err := client.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{
			UserId: "usr_wire_test",
		})
		if err != nil {
			t.Fatalf("CheckActiveOrders failed: %v", err)
		}

		jsonBytes, _ := formatter.Marshal(resp)
		t.Logf("Response (Active Orders Found):\n%s", string(jsonBytes))

		if !resp.GetHasActiveOrders() {
			t.Errorf("Expected HasActiveOrders true, got false")
		}
		if resp.GetActiveOrderCount() != 2 {
			t.Errorf("Expected ActiveOrderCount 2, got %d", resp.GetActiveOrderCount())
		}
		if len(resp.GetActiveOrders()) != 2 {
			t.Fatalf("Expected 2 active orders, got %d", len(resp.GetActiveOrders()))
		}

		first := resp.GetActiveOrders()[0]
		if first.GetOrderId() != "ord_wire_1" || first.GetStatus() != orderv1.OrderStatus_ORDER_STATUS_PAID {
			t.Errorf("First order mismatch: %+v", first)
		}

		second := resp.GetActiveOrders()[1]
		if second.GetOrderId() != "ord_wire_2" || second.GetStatus() != orderv1.OrderStatus_ORDER_STATUS_SHIPPED {
			t.Errorf("Second order mismatch: %+v", second)
		}
	})

	t.Run("successful e2e CheckActiveOrders with no active orders", func(t *testing.T) {
		emptyRepo := &mockOrderRepo{activeOrders: []db.OrderModel{}}
		emptyClient, emptyCleanup := setupTestGRPCServer(t, emptyRepo)
		defer emptyCleanup()

		resp, err := emptyClient.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{
			UserId: "usr_no_orders",
		})
		if err != nil {
			t.Fatalf("CheckActiveOrders failed: %v", err)
		}

		jsonBytes, _ := formatter.Marshal(resp)
		t.Logf("Response (No Active Orders):\n%s", string(jsonBytes))

		if resp.GetHasActiveOrders() {
			t.Errorf("Expected HasActiveOrders false, got true")
		}
		if resp.GetActiveOrderCount() != 0 {
			t.Errorf("Expected ActiveOrderCount 0, got %d", resp.GetActiveOrderCount())
		}
	})

	t.Run("e2e CheckActiveOrders with empty user_id validation error", func(t *testing.T) {
		_, err := client.CheckActiveOrders(ctx, &orderv1.CheckActiveOrdersRequest{
			UserId: "",
		})
		if err == nil {
			t.Fatal("Expected error for empty user ID, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got %v", err)
		}
		t.Logf("Response (Validation Error):\nCode: %v (%d)\nMessage: %s", st.Code(), st.Code(), st.Message())
		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument code, got %v", st.Code())
		}
		if st.Message() != "user_id is required" {
			t.Errorf("Expected message 'user_id is required', got '%s'", st.Message())
		}
	})
}
