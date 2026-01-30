package main

import (
	"context"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

func (s *server) GetState(ctx context.Context, req *api.GetStateRequest) (*api.GetStateResponse, error) {
	return &api.GetStateResponse{
		State: s.currentState,
	}, nil
}

func (s *server) PostAction(ctx context.Context, req *api.PostActionRequest) (*api.PostActionResponse, error) {
	// TODO: verify the player that submitted the action is owner of the city
	return &api.PostActionResponse{}, nil
}
