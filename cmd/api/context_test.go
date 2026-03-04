package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/usmanzaheer1995/greenlight/internal/assert"
	"github.com/usmanzaheer1995/greenlight/internal/data"
)

func TestContextSetUser(t *testing.T) {
	app := &application{}

	user := &data.User{
		ID:    1,
		Email: "test@example.com",
		Name:  "Test User",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = app.contextSetUser(req, user)

	result := req.Context().Value(userContextKey).(*data.User)
	assert.Equal(t, result, user)
}

func TestContextGetUser(t *testing.T) {
	app := &application{}

	t.Run("returns user when set", func(t *testing.T) {
		user := &data.User{
			ID:    1,
			Email: "test@example.com",
			Name:  "Test User",
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = app.contextSetUser(req, user)

		retrievedUser := app.contextGetUser(req)

		assert.Equal(t, retrievedUser, user)
	})

	t.Run("handles anonymous user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = app.contextSetUser(req, data.AnonymousUser)

		retrievedUser := app.contextGetUser(req)

		assert.Equal(t, retrievedUser.IsAnonymous(), false)
	})

	t.Run("panics when user not set", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("contextGetUser: expected panic when user not in context, but did not panic")
			}
		}()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		app.contextGetUser(req) // should panic
	})
}
