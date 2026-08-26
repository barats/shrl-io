package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/barats/shrl-io/internal/domain"
	"github.com/barats/shrl-io/internal/store"
)

type ctxKey int

const userKey ctxKey = 0

// internalSecretHeader carries the shared secret the frontend presents on
// every proxied request (ADR 0015). The api is reachable only on the internal
// network; this header is defense-in-depth, not user auth.
const internalSecretHeader = "X-Shrl-Internal-Secret"

// internalHeader rejects every request that lacks the shared frontend secret,
// including /login and /logout, which only ever arrive proxied by the
// frontend server.
func (s *server) internalHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(internalSecretHeader)
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.internalSecret)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func currentUser(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userKey).(*domain.User)
	return u
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// authenticate resolves a bearer credential to a user. The credential may be
// a login Token (TTL-bounded) or an API Key (no expiry); both share the
// Authorization: Bearer path and the SHA-256 hash lookup.
func (s *server) authenticate(ctx context.Context, token string) (*domain.User, error) {
	hash := domain.HashToken(token)
	if t, err := s.users.TokenByHash(ctx, hash); err == nil {
		if time.Now().After(t.ExpiresAt) {
			return nil, store.ErrNotFound
		}
		return s.users.GetByID(ctx, t.UserID)
	}
	if k, err := s.users.KeyByHash(ctx, hash); err == nil {
		return s.users.GetByID(ctx, k.UserID)
	}
	return nil, store.ErrNotFound
}

// auth guards every route except login/logout with a bearer token. A user
// flagged must-change-password (temp password from an admin reset) may only
// change their password or inspect their own account until it is replaced.
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" || r.URL.Path == "/logout" {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		u, err := s.authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if u.MustChangePassword && r.URL.Path != "/account/password" && r.URL.Path != "/me" {
			writeError(w, http.StatusForbidden, "password change required")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := s.users.GetByUsername(r.Context(), req.Username)
	if err != nil || !domain.VerifyPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, err := domain.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}
	t := &domain.Token{
		UserID:    u.ID,
		Hash:      domain.HashToken(token),
		ExpiresAt: time.Now().Add(s.cfg.tokenTTL),
	}
	if err := s.users.CreateToken(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	t, err := s.users.TokenByHash(r.Context(), domain.HashToken(token))
	if err == nil {
		_ = s.users.DeleteToken(r.Context(), t.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}
