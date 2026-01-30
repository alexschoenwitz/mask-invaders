package main

import (
	"context"
	"net"
	"net/http"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// --- The gRPC Server ---
type server struct {
	api.UnimplementedServerServer
}

func (s *server) GetState(ctx context.Context, req *api.GetStateRequest) (*api.GetStateResponse, error) {
	return &api.GetStateResponse{Greeting: "Hello"}, nil
}

func main() {
	// 1. Start gRPC Server (Binary)
	lis, _ := net.Listen("tcp", ":9090")
	s := grpc.NewServer()
	api.RegisterServerServer(s, &server{})
	go s.Serve(lis)

	// 2. Start Gateway Proxy (JSON -> gRPC)
	conn, _ := grpc.DialContext(
		context.Background(),
		"0.0.0.0:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	gwmux := runtime.NewServeMux()
	api.RegisterServerHandler(context.Background(), gwmux, conn)

	// Serve the JSON API on port 8080
	http.ListenAndServe(":8080", gwmux)
}
