package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jeffreyyong/payment-gateway/internal/app/listeners/httplistener"
	"github.com/jeffreyyong/payment-gateway/internal/domain"
)

const (
	endpointHelloWorld = "/hello-world"
)

// Service represents an interface for a service layer allowing HTTP routing logic and business logic to be separated
type Service interface {
	Authorize(ctx context.Context, payment *domain.Authorization) (*domain.Transaction, error)
}

// httpHandler is the http handler that will enable
// calls to this service via HTTP REST
type httpHandler struct {
	service Service
}

// NewHTTPHandler will create a new instance of httpHandler
func NewHTTPHandler(service Service) (*httpHandler, error) {
	if service == nil {
		return nil, fmt.Errorf("%w: service", errors.New("some error"))
	}

	return &httpHandler{
		service: service,
	}, nil
}

// ApplyRoutes will link the HTTP REST endpoint to the corresponding function in this handler
func (h *httpHandler) ApplyRoutes(m *httplistener.Mux) {
	m.HandleFunc(endpointHelloWorld, h.Capture)
}

func (h *httpHandler) Capture(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
