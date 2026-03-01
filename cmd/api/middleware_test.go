package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/usmanzaheer1995/greenlight/internal/jsonlog"
)

func TestRecoverPanic(t *testing.T) {
	app := &application{
		logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
	}

	t.Run("passes through normal request", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		app.recoverPanic(next).ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("recovers from panic and returns 500", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went wrong")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		app.recoverPanic(next).ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		if rr.Header().Get("Connection") != "close" {
			t.Errorf("expected Connection: close header to be set")
		}
	})

	t.Run("recovers from panic with error type", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app.errorResponse(w, r, http.StatusInternalServerError, fmt.Errorf("some error"))
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		app.recoverPanic(next).ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

func TestRateLimit(t *testing.T) {
	t.Run("disabled limiter passes all requests", func(t *testing.T) {
		app := &application{
			logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
			config: config{
				limiter: limiter{
					enabled: false,
				},
			},
		}

		next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		handler := app.rateLimit(next)

		for i := 0; i < 20; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.1:1234"
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("request %d: expected status: %d, actual: %d", i, http.StatusOK, rr.Code)
			}
		}
	})

	t.Run("allows requests within rate limit", func(t *testing.T) {
		app := &application{
			logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
			config: config{
				limiter: limiter{
					rps:     10,
					burst:   5,
					enabled: true,
				},
			},
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := app.rateLimit(next)

		for i := 0; i < 5; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.2:1234"
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("request %d: got status %d, want %d", i, rr.Code, http.StatusOK)
			}
		}
	})

	t.Run("blocks requests exceeding rate limit", func(t *testing.T) {
		app := &application{
			logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
			config: config{
				limiter: limiter{
					rps:     1,
					burst:   1,
					enabled: true,
				},
			},
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := app.rateLimit(next)

		gotRateLimited := false

		// With burst=1, second request should be rate limited
		for i := 0; i < 5; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.3:1234"
			handler.ServeHTTP(rr, req)

			if rr.Code == http.StatusTooManyRequests {
				gotRateLimited = true
				break
			}
		}

		if !gotRateLimited {
			t.Error("expected at least one request to be rate limited, but none were")
		}
	})

	t.Run("different IPs have independent limits", func(t *testing.T) {
		app := &application{
			logger: jsonlog.New(io.Discard, jsonlog.LevelInfo),
			config: config{
				limiter: struct {
					rps     float64
					burst   int
					enabled bool
				}{
					rps:     1,
					burst:   1,
					enabled: true,
				},
			},
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := app.rateLimit(next)

		// Exhaust the limit for IP A
		for i := 0; i < 5; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			handler.ServeHTTP(rr, req)
		}

		// IP B should still get through with its own fresh bucket
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("independent IP got status %d, want %d", rr.Code, http.StatusOK)
		}
	})
}
