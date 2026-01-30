package main

import (
	"context"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	playerIDKey      string = "playerID"
	isRegisterCtxKey string = "register"
)

type player struct {
	name  string
	token string
}

func (s *server) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	if !s.isAuthorized(ctx) {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

	if len(s.players) >= 16 {
		return nil, status.Error(codes.InvalidArgument, "no more players allowed")
	}
	token := uuid.NewString()
	s.players[token] = &player{
		name: req.Name,
	}
	return &api.RegisterResponse{
		Token: token,
	}, nil
}

func serverInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no metadata in context")
	}

	authHeaders, ok := md["authorization"]
	if ok && len(authHeaders) > 0 {
		ctx = context.WithValue(ctx, playerIDKey, authHeaders[0])
	}

	ctx = context.WithValue(ctx, isRegisterCtxKey, info.FullMethod == api.Server_Register_FullMethodName)

	return handler(ctx, req)
}

func (s *server) isAuthorized(ctx context.Context) bool {
	if ctx.Value(isRegisterCtxKey).(bool) {
		return true
	}

	playerID, ok := ctx.Value(playerIDKey).(string)
	if !ok {
		return false
	}
	for k := range s.players {
		if k == playerID {
			return true
		}
	}
	return false
}

func (s *server) playerID(ctx context.Context) (string, bool) {
	playerID, ok := ctx.Value(playerIDKey).(string)
	if !ok {
		return "", false
	}
	return playerID, true
}
