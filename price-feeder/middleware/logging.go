package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/0xcatalysis/core/go-sdk/log"
	"github.com/0xcatalysis/core/go-sdk/z"
)

type LoggingMiddleware struct{}

func NewLoggingMiddleware() *LoggingMiddleware {
	return &LoggingMiddleware{}
}

func (lm *LoggingMiddleware) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := context.Background()

		wrapper := &responseWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		// Log based on status code severity:
		// 5xx: Server errors (WARN - these are our problems)
		// 404: Not found (DEBUG - client trying non-existent routes, not our problem)
		// 4xx: Other client errors (INFO - useful to track but not alarming)
		// 2xx/3xx: Success (DEBUG - reduce verbosity)
		if wrapper.statusCode >= 500 {
			log.Warn(ctx, "HTTP request completed with server error", nil,
				z.Str("method", r.Method),
				z.Str("path", r.URL.Path),
				z.Int("status", wrapper.statusCode),
				z.Any("duration", duration),
				z.Str("remote_addr", r.RemoteAddr),
			)
		} else if wrapper.statusCode == 404 {
			log.Debug(ctx, "HTTP request not found",
				z.Str("method", r.Method),
				z.Str("path", r.URL.Path),
				z.Int("status", wrapper.statusCode),
				z.Str("remote_addr", r.RemoteAddr),
			)
		} else if wrapper.statusCode >= 400 {
			log.Info(ctx, "HTTP request completed with client error",
				z.Str("method", r.Method),
				z.Str("path", r.URL.Path),
				z.Int("status", wrapper.statusCode),
				z.Any("duration", duration),
			)
		} else {
			log.Debug(ctx, "HTTP request completed",
				z.Str("method", r.Method),
				z.Str("path", r.URL.Path),
				z.Int("status", wrapper.statusCode),
				z.Any("duration", duration),
			)
		}
	})
}

type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RateLimitMiddleware(requestsPerSecond int) func(http.Handler) http.Handler {
	tokens := make(chan struct{}, requestsPerSecond)

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
		defer ticker.Stop()

		for range ticker.C {
			select {
			case tokens <- struct{}{}:
			default:
			}
		}
	}()

	for range requestsPerSecond {
		tokens <- struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-tokens:
				next.ServeHTTP(w, r)
			default:
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			}
		})
	}
}

func HealthCheckMiddleware(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path && r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)); err != nil {
					http.Error(w, "Failed to write health response", http.StatusInternalServerError)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
