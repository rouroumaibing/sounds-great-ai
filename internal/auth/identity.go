// Package auth implements the multi-user identity and authorization layer that
// replaces the single-token engine (roadmap P1-A, README#10). It provides:
//   - multi-user identity via bearer tokens (identity.go)
//   - per-thread access control lists with fail-closed checks (acl.go)
//   - callback auth lifecycle (callback.go)
//   - persistent agent keys with rotate/revoke (agentkey.go)
//
// Every check is fail-closed: an unknown principal, missing token, expired
// credential, or absent ACL rule denies by default.
package auth

import (
	"errors"
	"sync"

	"sounds-great-ai/internal/context"
)

// User is a human operator identity.
type User struct {
	ID    string
	Name  string
	Roles []string
}

// ErrUnauthenticated is returned when a token is missing or unknown.
var ErrUnauthenticated = errors.New("auth: unauthenticated (fail-closed)")

// TokenStore maps bearer tokens to users. It is the simplest identity backend;
// production may back it with a DB. Goroutine-safe.
type TokenStore struct {
	mu    sync.RWMutex
	tokens map[string]User
}

// NewTokenStore creates an empty store.
func NewTokenStore() *TokenStore {
	return &TokenStore{tokens: make(map[string]User)}
}

// Put registers (or replaces) a token->user mapping.
func (s *TokenStore) Put(token string, u User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = u
}

// Authenticate resolves a bearer token to a Principal. Fail-closed: an empty
// or unknown token yields ErrUnauthenticated.
func (s *TokenStore) Authenticate(token string) (*context.Principal, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	s.mu.RLock()
	u, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrUnauthenticated
	}
	return &context.Principal{
		ID:   u.ID,
		Type: context.PrincipalUser,
		Name: u.Name,
	}, nil
}

// Identities returns the registered users (for diagnostics).
func (s *TokenStore) Identities() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.tokens))
	for _, u := range s.tokens {
		out = append(out, u)
	}
	return out
}
