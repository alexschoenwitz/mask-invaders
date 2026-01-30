package main

import (
	"sync"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

type mage struct {
	currentState *api.State
	stateHistory []*api.State
	stateLock    sync.Mutex
}

func (m *mage) generateState() *api.State {
	return m.currentState
}

func (m *mage) state() *api.State {
	return m.currentState
}
