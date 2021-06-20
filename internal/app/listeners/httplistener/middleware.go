package httplistener

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	appcontext "github.com/jeffreyyong/payment-gateway/internal/app/context"
	"github.com/jeffreyyong/payment-gateway/internal/logging"
)

// MiddlewareFunc defines a middleware type
type MiddlewareFunc func(h http.HandlerFunc) http.HandlerFunc

// APIMiddleware adds the api to the context
func APIMiddleware(api string) MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := appcontext.WithAPI(r.Context(), api)
			rr := r.WithContext(ctx)
			next(w, rr)
		}
	}
}

// ContextMiddleware adds service context to zapcontext and the context
func ContextMiddleware() MiddlewareFunc {
	service := os.Getenv("SERVICE")

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = logging.WithFields(ctx,
				zap.String("service", service),
			)
			ctx = appcontext.WithService(ctx, service)

			rr := r.WithContext(ctx)
			next(w, rr)
		}
	}
}

type responseRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (l *responseRecorder) WriteHeader(code int) {
	l.StatusCode = code
	l.ResponseWriter.WriteHeader(code)
}

// Hijack implements the http.Hijacker interface
func (l *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := l.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("Hijack was called on the response but it does not implement the http.Hijacker interface")
	}
	return hj.Hijack()
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
	}
}

// LoggingMiddleware logs the request and response events using the default logger and Jaeger
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		u := r.URL.String()
		api := appcontext.GetAPI(r.Context())

		fields := []zap.Field{
			zap.String("api", api),
			zap.String("middleware", "app_httplistener"),
			zap.String("protocol", "http"),
			zap.String("http.method", r.Method),
			zap.String("http.url", u),
		}
		logging.Print(r.Context(), "app__httplistener__request_received", fields...)

		lw := newResponseRecorder(w)
		next(lw, r)

		fields = append(fields,
			zap.String("middleware", "app_httplistener"),
			zap.String("duration", time.Since(start).String()),
			zap.Int("http.status_code", lw.StatusCode),
		)
		logging.Print(r.Context(), "app__httplistener__response_sent", fields...)
	}
}
