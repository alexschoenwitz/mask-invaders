package main

import (
	"context"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type player struct {
	name  string
	token string
}

func (s *server) Register(ctx context.Context, req *api.RegisterRequest) (*api.RegisterResponse, error) {
	if len(s.players) >= 16 {
		return nil, status.Error(codes.InvalidArgument, "no more players allowed")
	}
	token := uuid.NewString()
	s.players = append(s.players, &player{
		name:  req.Name,
		token: token,
	})
	return &api.RegisterResponse{
		Token: token,
	}, nil
}
