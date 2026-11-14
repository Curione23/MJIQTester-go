package main

import (
	"sync"
	"time"
)

type VerificationState struct {
	mu        sync.Mutex
	active    map[string]struct{}
	cooldowns map[string]time.Time
}

func NewVerificationState() *VerificationState {
	s := &VerificationState{
		active:    map[string]struct{}{},
		cooldowns: map[string]time.Time{},
	}
	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for range t.C {
			s.prune()
		}
	}()
	return s
}

func (s *VerificationState) prune() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, exp := range s.cooldowns {
		if !exp.After(now) {
			delete(s.cooldowns, id)
		}
	}
}

func (s *VerificationState) IsInProgress(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.active[userID]
	return ok
}

func (s *VerificationState) StartQuiz(userID string) { s.mu.Lock(); s.active[userID] = struct{}{}; s.mu.Unlock() }
func (s *VerificationState) EndQuiz(userID string)   { s.mu.Lock(); delete(s.active, userID); s.mu.Unlock() }

func (s *VerificationState) GetCooldownRemaining(userID string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.cooldowns[userID]
	if !ok {
		return 0
	}
	rem := time.Until(exp)
	if rem <= 0 {
		delete(s.cooldowns, userID)
		return 0
	}
	return rem
}

func (s *VerificationState) SetCooldown(userID string, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d <= 0 {
		delete(s.cooldowns, userID)
		return
	}
	s.cooldowns[userID] = time.Now().Add(d)
}

func (s *VerificationState) ClearCooldown(userID string) {
	s.mu.Lock()
	delete(s.cooldowns, userID)
	s.mu.Unlock()
}
