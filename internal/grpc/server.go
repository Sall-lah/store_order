package grpc

import (
	"fmt"
	"log"
	"net"

	orderv1 "github.com/Sall-lah/store_proto/gen/go/store/order/v1"
	"google.golang.org/grpc"
)

// Server wraps the underlying gRPC server instance and network listener configuration.
type Server struct {
	grpcServer *grpc.Server
	port       string
	listener   net.Listener
}

// NewServer initializes a configured gRPC server instance and registers the OrderService service.
// Why: Provides a standardized constructor that encapsulates RPC handler binding and middleware interceptors.
func NewServer(port string, orderServiceServer *OrderServiceServer, opts ...grpc.ServerOption) *Server {
	grpcServer := grpc.NewServer(opts...)
	orderv1.RegisterOrderServiceServer(grpcServer, orderServiceServer)

	return &Server{
		grpcServer: grpcServer,
		port:       port,
	}
}

// Start begins listening on the configured TCP port and serving incoming gRPC requests.
// Why: Manages the TCP socket lifecycle and blocks until the server stops or encounters a fatal listener error.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind gRPC listener on %s: %w", addr, err)
	}
	s.listener = listener

	log.Printf("[INFO] gRPC server listening on port %s", s.port)
	if err := s.grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
		return fmt.Errorf("gRPC server serve error: %w", err)
	}

	return nil
}

// Stop gracefully terminates the gRPC server, allowing active in-flight RPCs to complete.
// Why: Ensures zero dropped requests or uncoordinated client terminations during service rollout.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		log.Println("[INFO] Stopping gRPC server gracefully...")
		s.grpcServer.GracefulStop()
		log.Println("[INFO] gRPC server stopped.")
	}
}
