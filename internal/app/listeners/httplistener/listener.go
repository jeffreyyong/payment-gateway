package httplistener

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	gmux "github.com/gorilla/mux"
	"gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"

	"github.com/jeffreyyong/payment-gateway/internal/logging"
)

type Listener struct {
	server                   *http.Server
	handler                  Handler
	addr                     string
	isRequestLoggingDisabled bool
}

type handlerOptions struct {
	isRequestLoggingDisabled bool
}

type handlerOptsFunc func(h *handlerOptions)

func withRequestLoggingDisabled() handlerOptsFunc {
	return func(h *handlerOptions) { h.isRequestLoggingDisabled = true }
}

// HTTPHandler is exposed for convince for writing tests, it should
// not be used for creating HTTP servers directly.
func HTTPHandler(h Handler, opts ...handlerOptsFunc) http.Handler {
	t := mux.NewRouter()
	m := &Mux{t.Router}
	h.ApplyRoutes(m)

	opt := &handlerOptions{}
	for _, f := range opts {
		f(opt)
	}

	_ = t.Walk(func(route *gmux.Route, router *gmux.Router, ancestors []*gmux.Route) error {
		h := route.GetHandler()
		if h == nil {
			return nil
		}

		name := route.GetName()
		if name == "" {
			var err error
			name, err = route.GetPathTemplate()
			if err != nil {
				name = "/"
			}
		}

		middleware := []MiddlewareFunc{
			APIMiddleware(name),
			ContextMiddleware(),
		}
		if !opt.isRequestLoggingDisabled {
			middleware = append(middleware, LoggingMiddleware) // depends on SpanMiddleware and ContextMiddleware
		}

		f := h.ServeHTTP
		for i := len(middleware) - 1; i >= 0; i-- {
			m := middleware[i]
			f = m(f)
		}
		route.HandlerFunc(f)
		return nil
	})

	return t
}

type Mux struct {
	*gmux.Router
}

type Route = gmux.Route

// Group groups a subrouter and allows for grouping together routes
// which share a common pattern.
func (m *Mux) Group(route *Route, fn func(m *Mux)) {
	s := route.Subrouter()
	fn(&Mux{s})
}

type Handler interface {
	ApplyRoutes(m *Mux)
}

type Option func(*Listener)

func WithAddr(addr string) Option {
	return func(l *Listener) { l.addr = addr }
}

// WithRequestLoggingDisabled explicitly disables the logging middleware.
func WithRequestLoggingDisabled() Option {
	return func(l *Listener) { l.isRequestLoggingDisabled = true }
}

func New(h Handler, opts ...Option) *Listener {
	l := &Listener{
		server: &http.Server{
			BaseContext: func(net.Listener) context.Context {
				return logging.With(context.Background(), logging.From(context.Background()))
			},
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		handler: h,

		addr: ":8080",
	}

	for _, opt := range opts {
		opt(l)
	}

	l.server.Addr = l.addr

	return l
}

func (l *Listener) Name() string { return "http" }

func (l *Listener) Serve(ctx context.Context) error {
	// creating this here so that the DataDog tracing setup pulls in
	// the service name from the global config. If it was done in New
	// it is possible for users to call it before app.Run has been called
	// which is what will setup the DataDog tracer and inturn its global config.
	// This means users will not get their traces in APM and not be sure why.

	opts := []handlerOptsFunc{}
	if l.isRequestLoggingDisabled {
		opts = append(opts, withRequestLoggingDisabled())
	}

	l.server.Handler = HTTPHandler(l.handler, opts...)
	if err := l.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	return nil
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (l *Listener) Close(ctx context.Context) error {
	if err := l.server.Shutdown(ctx); err != nil {
		if isContextErr(err) {
			l.server.Close()
		}
		return err
	}
	return nil
}
