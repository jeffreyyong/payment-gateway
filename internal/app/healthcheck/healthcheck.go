package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/jeffreyyong/payment-gateway/internal/logging"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Health check related errors.
var (
	ErrHTTPStatus = errors.New("bad HTTP status")
)

// Checker defines the health checker interface.
type Checker interface {
	Health(context.Context) *Service
}

// Pinger defines a ping interface.
type Pinger interface {
	Ping(context.Context) error
}

// Response represents a health check response.
type Response struct {
	Healthy  bool       `json:"healthy"`
	Services []*Service `json:"services,omitempty"`
}

// Service represents a individual
type Service struct {
	Name    string  `json:"name"`
	Error   string  `json:"error,omitempty"`
	Latency float64 `json:"latency,omitempty"`
	Healthy bool    `json:"healthy"`
}

// DefaultChecker represents a default checker helper implementation.
type DefaultChecker struct {
	name  string
	check func(context.Context) error
}

// Doer defines the HTTP standard library Do() method.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewDefaultChecker returns a new default health
// checker with the given name and check function.
func NewDefaultChecker(name string, check func(context.Context) error) *DefaultChecker {
	return &DefaultChecker{name: name, check: check}
}

// Health implements the Checker interface.
func (c *DefaultChecker) Health(ctx context.Context) *Service {
	ctx = logging.WithFields(ctx, zap.String("dependency", c.name))

	service := &Service{Name: c.name}
	start := time.Now()

	err := c.check(ctx)
	service.Healthy = err == nil

	service.Latency = time.Since(start).Seconds()

	if err != nil {
		logging.Error(ctx, "service dependency is not healthy", zap.Error(err))
		service.Error = err.Error()
	}

	return service
}

// CheckAll checks all services' health returning a health check response.
func CheckAll(ctx context.Context, checkers ...Checker) *Response {
	resp := &Response{Healthy: true}
	if len(checkers) == 0 {
		// No dependencies, healthy by default
		return resp
	}

	var (
		wg      sync.WaitGroup
		results = make(chan *Service)
		done    = make(chan struct{})
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for v := range results {
			resp.Services = append(resp.Services, v)
			resp.Healthy = resp.Healthy && v.Healthy
			if !v.Healthy {
				cancel()
				break
			}
		}

		done <- struct{}{}
	}()

	for _, v := range checkers {
		wg.Add(1)

		go func(checker Checker) {
			defer wg.Done()

			select {
			case results <- checker.Health(ctx):
			case <-ctx.Done():
				return
			}
		}(v)
	}

	wg.Wait()
	close(results)

	<-done

	return resp
}

// Handler returns a http.Handler which will check the status of provided
// checkers. If the service is deemed unhealthy, the server responds with
// http.StatusServiceUnavailable and if the request method is not HEAD, it will
// write the statuses as a JSON body.
func Handler(checkers ...Checker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := CheckAll(r.Context(), checkers...)
		if !status.Healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if r.Method != http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(status)
		}
	})
}
