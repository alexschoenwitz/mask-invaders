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

type contextKey string

const (
	playerTokenKey contextKey = "playerToken"
)

type player struct {
	id   string
	name string
}

func (s *server) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	g, ok := s.getGame(req.GameId)
	if !ok {
		return nil, status.Error(codes.NotFound, "game not found")
	}

	g.playersLock.Lock()
	defer g.playersLock.Unlock()

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name cannot be empty")
	}

	if g.started.Load() {
		return nil, status.Error(codes.FailedPrecondition, "game already started")
	}

	if len(g.players) >= 16 {
		return nil, status.Error(codes.InvalidArgument, "no more players allowed")
	}

	id := uuid.NewString()
	token := uuid.NewString()

	g.players[token] = &player{
		name: req.Name,
		id:   id,
	}
	return &api.RegisterResponse{
		Token: token,
		Id:    id,
	}, nil
}

var unauthenticatedMethods = map[string]struct{}{
	api.Server_Register_FullMethodName:        {},
	api.Server_GetStateHistory_FullMethodName: {},
	api.Server_GetState_FullMethodName:        {},
	api.Server_ListGames_FullMethodName:       {},
	api.Server_CreateGame_FullMethodName:      {},
}

func serverInterceptor(ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	type GameScopedRequest interface {
		GetGameId() string
	}

	s, ok := info.Server.(*server)
	if !ok {
		return nil, status.Error(codes.Internal, "what happened to the server?")
	}

	// Bypass auth for unauthenticated methods
	if _, ok := unauthenticatedMethods[info.FullMethod]; ok {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no metadata in context")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) > 0 {
		ctx = context.WithValue(ctx, playerTokenKey, authHeaders[0])
	}

	p, ok := req.(GameScopedRequest)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "permission denied 1")
	}

	id := p.GetGameId()
	if !isAuthorized(s, id, ctx) {
		return nil, status.Error(codes.PermissionDenied, "permission denied 2")
	}

	return handler(ctx, req)
}

func isAuthorized(s *server, gameId string, ctx context.Context) bool {
	playerToken, playerExists := ctx.Value(playerTokenKey).(string)
	if !playerExists {
		return false
	}

	s.gamesLock.RLock()
	g, gameExists := s.games[gameId]
	s.gamesLock.RUnlock()

	if !gameExists {
		return false
	}

	g.playersLock.RLock()
	defer g.playersLock.RUnlock()
	_, playerExists = g.players[playerToken]
	return playerExists
}

func getPlayerToken(ctx context.Context) (string, bool) {
	playerID, ok := ctx.Value(playerTokenKey).(string)
	if !ok {
		return "", false
	}
	return playerID, true
}
