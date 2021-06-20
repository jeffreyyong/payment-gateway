package app

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/app/healthcheck"
	"github.com/jeffreyyong/payment-gateway/internal/app/listeners/httplistener"
	"github.com/jeffreyyong/payment-gateway/internal/logging"
)

type Listener interface {
	Serve(ctx context.Context) error
	Close(ctx context.Context) error
	Name() string
}

type hooks struct {
	postHealth func()
}

type Service struct {
	name              string
	opts              options
	onShutdown        []func()
	readinessCheckers []healthcheck.Checker
	livenessCheckers  []healthcheck.Checker
	healthHandler     healthHandler
}

type options struct {
	version         string
	sha             string
	timeout         time.Duration
	healthAddr      string
	shutdownTimeout time.Duration

	hooks hooks
}

type Option func(*options)

func defaultOpts() options {
	return options{
		timeout:         1 * time.Minute,
		healthAddr:      ":8082",
		shutdownTimeout: 5 * time.Second,
	}
}

type SetupFunc func(ctx context.Context, service *Service) ([]Listener, context.Context, error)

func Context() context.Context {
	l := logging.From(context.Background())
	return logging.With(context.Background(), l)
}

func Run(name string, setup SetupFunc, opts ...Option) error {
	options := defaultOpts()
	s := &Service{
		name: name,
		opts: options,
	}

	for _, opt := range opts {
		opt(&s.opts)
	}

	return s.run(Context(), setup)
}

func (s *Service) earlyInit(ctx context.Context, errs chan<- error) []Listener {
	health := httplistener.New(&s.healthHandler, httplistener.WithAddr(s.opts.healthAddr), httplistener.WithRequestLoggingDisabled())
	s.launch(ctx, health, errs)
	return []Listener{health}
}
func (s *Service) launch(ctx context.Context, listener Listener, errs chan<- error) {
	// this doesnt use an errgroup because in an errgroup its wait only returns once
	// all the functions return, for us that would mean manually unblocking the serve
	// method on the listener. Instead I wanted to have the listeners run, the first one
	// to exit triggers the service to exit and handles graceful shutdown of the other
	// listeners.
	go func() {
		err := listener.Serve(ctx)
		logging.From(ctx).Warn("listener exited", zap.String("name", listener.Name()), zap.Error(err))

		select {
		case errs <- err:
		default:
		}
	}()
}

func (s *Service) shutdown(ctx context.Context, listeners []Listener) {
	ctx, cancel := context.WithTimeout(ctx, s.opts.shutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, l := range listeners {
		wg.Add(1)
		l := l
		go func() {
			defer wg.Done()
			if err := l.Close(ctx); err != nil {
				logging.From(ctx).Error("error shutting down listener", zap.Error(err), zap.String("listener", l.Name()))
			}
		}()
	}

	wg.Wait()
}

func (s *Service) setupListeners(ctx context.Context, setupFunc SetupFunc) ([]Listener, context.Context, error) {
	type listenerSetup struct {
		listeners []Listener
		ctx       context.Context
		err       error
	}

	setup := make(chan listenerSetup, 1)
	go func() {
		listeners, ctx, err := func() (_ []Listener, _ context.Context, err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}

				if v, ok := r.(error); ok {
					err = fmt.Errorf("app: panic during listener setup: %w", v)
				} else {
					err = fmt.Errorf("app: panic during listener setup: %v", r)
				}
				// not re-panicing here as were going to let app exit normally
				logging.From(ctx).Error("panic during listener setup", zap.Stack("stack"), zap.Any("panic", r))
			}()

			return setupFunc(ctx, s)
		}()

		setup <- listenerSetup{listeners, ctx, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx, fmt.Errorf("app: listener setup timedout: %w", ctx.Err())
	case resp := <-setup:
		if resp.ctx != nil {
			ctx = resp.ctx
		}
		return resp.listeners, ctx, resp.err
	}
}

func (s *Service) wait(ctx context.Context, errs <-chan error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	select {
	case err := <-errs:
		return err
	case s := <-sigs:
		logging.From(ctx).Info("received signal, shutting down", zap.Stringer("signal", s))
		return nil
	case <-ctx.Done():
		select {
		case err := <-errs:
			return err
		default:
			return ctx.Err()
		}
	}
}

func (s *Service) run(ctx context.Context, setupFunc SetupFunc) error {
	defer func() {
		for _, fn := range s.onShutdown {
			fn()
		}
	}()
	logger := logging.From(context.Background()).With(
		zap.String("sha", s.opts.sha),
		zap.String("version", s.opts.version),
		zap.String("name", s.name))
	logger.Info("service starting")

	if _, err := maxprocs.Set(maxprocs.Logger(zap.NewStdLog(logger).Printf)); err != nil {
		return fmt.Errorf("app: unable to set GOMAXPROCS: %w", err)
	}

	logger.Info("set GOMAXPROCS", zap.Int("num_cpu", runtime.NumCPU()), zap.Int("GOMAXPROCS", runtime.GOMAXPROCS(0)))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 1)
	earlyListeners := s.earlyInit(ctx, errs)
	defer s.shutdown(Context(), earlyListeners)

	listeners, ctx, err := s.setupListeners(ctx, setupFunc)
	if err != nil {
		return err
	}
	defer s.shutdown(Context(), listeners)

	if len(listeners) == 0 {
		return nil
	}

	for _, l := range listeners {
		s.launch(ctx, l, errs)
	}

	// we are ready to launch!
	s.healthHandler.readiness.Store(healthcheck.Handler(s.readinessCheckers...))
	s.healthHandler.liveness.Store(healthcheck.Handler(s.livenessCheckers...))

	if fn := s.opts.hooks.postHealth; fn != nil {
		fn()
	}

	return s.wait(ctx, errs)
}

func (s *Service) OnShutdown(fn func()) *Service {
	s.onShutdown = append(s.onShutdown, fn)
	return s
}

type healthHandler struct {
	readiness atomic.Value // http.Handler
	liveness  atomic.Value // http.Handler
}

func (h *healthHandler) ApplyRoutes(m *httplistener.Mux) {
	m.HandleFunc("/_live", h.LivenessHandler)
	m.HandleFunc("/_health", h.ReadinessHandler)

	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.HandleFunc("/debug/pprof/trace", pprof.Trace)
	m.NewRoute().PathPrefix("/debug/pprof").HandlerFunc(pprof.Index)
	m.Handle("/debug/vars", expvar.Handler())
}
func (h *healthHandler) LivenessHandler(rw http.ResponseWriter, r *http.Request) {
	handler, _ := h.liveness.Load().(http.Handler)
	if handler == nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(rw, r)
}

func (h *healthHandler) ReadinessHandler(rw http.ResponseWriter, r *http.Request) {
	handler, _ := h.readiness.Load().(http.Handler)
	if handler == nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	handler.ServeHTTP(rw, r)
}
