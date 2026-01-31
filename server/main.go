package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type server struct {
	api.UnimplementedServerServer

	actionsLock sync.Mutex
	actionQueue chan *api.Action

	currentState *api.State

	playersLock sync.RWMutex
	players     map[string]*player

	stateLock    sync.RWMutex
	stateHistory []*api.State

	submittedActions map[string]*api.Action

	turnCount   int64
	gameStarted atomic.Bool // locked once started

	shutdown func()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(serverInterceptor))
	s := &server{
		submittedActions: make(map[string]*api.Action),
		actionQueue:      make(chan *api.Action, 100),
		players:          make(map[string]*player),
		shutdown:         cancel,
	}
	go s.run(ctx)
	api.RegisterServerServer(grpcServer, s)

	go func() {
		log.Println("gRPC server starting on :9090")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to dial gRPC server: %v", err)
	}
	defer conn.Close()

	gwmux := runtime.NewServeMux()
	if err := api.RegisterServerHandler(ctx, gwmux, conn); err != nil {
		log.Fatalf("failed to register gateway: %v", err)
	}

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      gwmux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("HTTP gateway starting on :8080")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to serve HTTP: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case <-ctx.Done():
	}

	log.Println("shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-time.After(30 * time.Second):
		grpcServer.Stop()
		log.Println("gRPC server force stopped")
	}

	log.Println("servers shutdown complete")
}
