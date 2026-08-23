package grpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sall-lah/store_order/internal/repository"
	orderv1 "github.com/Sall-lah/store_proto/gen/go/store/order/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderServiceServer implements the gRPC OrderServiceServer contract from store_proto.
type OrderServiceServer struct {
	orderv1.UnimplementedOrderServiceServer
	orderRepo repository.OrderRepository
}

// NewOrderServiceServer creates a new instance of OrderServiceServer backed by the order repository.
// Why: Injects repository persistence dependencies into the gRPC transport layer.
func NewOrderServiceServer(orderRepo repository.OrderRepository) *OrderServiceServer {
	return &OrderServiceServer{
		orderRepo: orderRepo,
	}
}

// CheckActiveOrders inspects whether a user has pending or in-flight orders blocking account deletion.
// Why: Serves as a synchronous pre-flight check for upstream services before finalizing destructive user account actions.
func (s *OrderServiceServer) CheckActiveOrders(
	ctx context.Context,
	req *orderv1.CheckActiveOrdersRequest,
) (*orderv1.CheckActiveOrdersResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	orders, err := s.orderRepo.ListActiveOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to query active orders for user: %v", err)
	}

	if len(orders) == 0 {
		return &orderv1.CheckActiveOrdersResponse{
			HasActiveOrders:  false,
			ActiveOrderCount: 0,
			ActiveOrders:     []*orderv1.ActiveOrderSummary{},
			Message:          "User has no active orders",
		}, nil
	}

	summaries := make([]*orderv1.ActiveOrderSummary, 0, len(orders))
	for _, order := range orders {
		summaries = append(summaries, MapOrderModelToActiveSummary(order))
	}

	return &orderv1.CheckActiveOrdersResponse{
		HasActiveOrders:  true,
		ActiveOrderCount: int32(len(orders)),
		ActiveOrders:     summaries,
		Message:          fmt.Sprintf("User has %d active order(s) blocking account deletion", len(orders)),
	}, nil
}
