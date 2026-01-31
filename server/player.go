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
	playerTokenKey string = "playerToken"
)

type player struct {
	id   string
	name string
}

func (s *server) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	s.playersLock.Lock()
	defer s.playersLock.Unlock()

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	if s.gameStarted.Load() {
		return nil, status.Error(codes.FailedPrecondition, "game already started")
	}

	if len(s.players) >= 16 {
		return nil, status.Error(codes.InvalidArgument, "no more players allowed")
	}

	id := uuid.NewString()
	token := uuid.NewString()

	s.players[token] = &player{
		name: req.Name,
		id:   id,
	}
	return &api.RegisterResponse{
		Token: token,
		Id:    id,
	}, nil
}

var (
	unauthenticatedMethods = map[string]struct{}{
		api.Server_Register_FullMethodName:        {},
		api.Server_GetStateHistory_FullMethodName: {},
		api.Server_GetState_FullMethodName:        {},
	}
)

func serverInterceptor(ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	// Bypass auth for register
	if info.FullMethod == api.Server_Register_FullMethodName {
		return handler(ctx, req)
	}

	s, ok := info.Server.(*server)
	if !ok {
		return nil, status.Error(codes.Internal, "what happened to the server?")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no metadata in context")
	}

	if _, ok := unauthenticatedMethods[info.FullMethod]; ok {
		return handler(ctx, req)
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) > 0 {
		ctx = context.WithValue(ctx, playerTokenKey, authHeaders[0])
	}
	if !isAuthorized(s, ctx) {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

	return handler(ctx, req)
}

func isAuthorized(s *server, ctx context.Context) bool {
	playerToken, playerExists := ctx.Value(playerTokenKey).(string)
	if !playerExists {
		return false
	}

	_, playerExists = s.players[playerToken]
	return playerExists
}

func getPlayerToken(ctx context.Context) (string, bool) {
	playerID, ok := ctx.Value(playerTokenKey).(string)
	if !ok {
		return "", false
	}
	return playerID, true
}
