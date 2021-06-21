//go:generate mockgen -destination=./mocks/handler_mock.go -package=mocks github.com/jeffreyyong/payment-gateway/internal/transport/transporthttp Service

package transporthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/app/listeners/httplistener"
	"github.com/jeffreyyong/payment-gateway/internal/domain"
	"github.com/jeffreyyong/payment-gateway/internal/logging"
)

const (
	EndpointAuthorize = "/authorize"
	EndpointCapture   = "/capture"
	EndpointRefund    = "/refund"
	EndpointVoid      = "/void"

	ContentType     = "Content-Type"
	ApplicationJSON = "application/json"
)

// Service represents an interface for a service layer allowing HTTP routing logic and business logic to be separated
type Service interface {
	Authorize(ctx context.Context, authorization *domain.Authorization) (*domain.Transaction, error)
	Capture(ctx context.Context, capture *domain.Capture) (*domain.Transaction, error)
	Refund(ctx context.Context, refund *domain.Refund) (*domain.Transaction, error)
	Void(ctx context.Context, void *domain.Void) (*domain.Transaction, error)
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
	m.HandleFunc(EndpointAuthorize, h.Authorize).Methods(http.MethodPost)
	m.HandleFunc(EndpointCapture, h.Capture).Methods(http.MethodPost)
	m.HandleFunc(EndpointRefund, h.Refund).Methods(http.MethodPost)
	m.HandleFunc(EndpointVoid, h.Void).Methods(http.MethodPost)
}

func (h *httpHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMsg := "error reading request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	if len(body) == 0 {
		errMsg := "missing request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	var req AuthorizeRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		errMsg := "failed to unmarshal request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	authorization := &domain.Authorization{
		RequestID: req.RequestID,
		PaymentSource: domain.PaymentSource{
			PAN: req.PaymentSource.PAN,
			CVV: req.PaymentSource.CVV,
			Expiry: domain.Expiry{
				Month: req.PaymentSource.ExpiryMonth,
				Year:  req.PaymentSource.ExpiryYear,
			},
		},
		Amount: domain.Amount{
			MinorUnits: req.Amount.MinorUnits,
			Currency:   req.Amount.Currency,
			Exponent:   req.Amount.Exponent,
		},
	}

	t, err := h.service.Authorize(ctx, authorization)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnprocessable):
			_ = WriteError(w, err.Error(), CodeUnprocessable)
			return
		default:
			errMsg := "failed to authorize transaction in service"
			_ = WriteError(w, errMsg, CodeUnknownFailure)
			return
		}
	}

	w.Header().Add(ContentType, ApplicationJSON)
	err = json.NewEncoder(w).Encode(mapToTransactionResp(t))
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}

func (h *httpHandler) Capture(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMsg := "error reading request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	if len(body) == 0 {
		errMsg := "missing request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	var req CaptureRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		errMsg := "failed to unmarshal request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	capture := &domain.Capture{
		RequestID:       req.RequestID,
		AuthorizationID: req.AuthorizationID,
		Amount: domain.Amount{
			MinorUnits: req.Amount.MinorUnits,
			Currency:   req.Amount.Currency,
			Exponent:   req.Amount.Exponent,
		},
	}

	t, err := h.service.Capture(ctx, capture)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransactionNotFound):
			errMsg := "unable to find the transaction with the authorization ID"
			_ = WriteError(w, errMsg, CodeNotFound)
			return
		case errors.Is(err, domain.ErrUnprocessable):
			_ = WriteError(w, err.Error(), CodeUnprocessable)
			return
		default:
			errMsg := "failed to capture transaction in service"
			_ = WriteError(w, errMsg, CodeUnknownFailure)
			return
		}
	}

	w.Header().Add(ContentType, ApplicationJSON)
	err = json.NewEncoder(w).Encode(mapToTransactionResp(t))
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}

func (h *httpHandler) Refund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMsg := "error reading request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	if len(body) == 0 {
		errMsg := "missing request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	var req RefundRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		errMsg := "failed to unmarshal request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	refund := &domain.Refund{
		RequestID:       req.RequestID,
		AuthorizationID: req.AuthorizationID,
		Amount: domain.Amount{
			MinorUnits: req.Amount.MinorUnits,
			Currency:   req.Amount.Currency,
			Exponent:   req.Amount.Exponent,
		},
	}

	t, err := h.service.Refund(ctx, refund)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransactionNotFound):
			errMsg := "unable to find the transaction with the authorization ID"
			_ = WriteError(w, errMsg, CodeNotFound)
			return
		case errors.Is(err, domain.ErrUnprocessable):
			_ = WriteError(w, err.Error(), CodeUnprocessable)
			return
		default:
			errMsg := "failed to refund transaction in service"
			_ = WriteError(w, errMsg, CodeUnknownFailure)
			return
		}
	}

	w.Header().Add(ContentType, ApplicationJSON)
	err = json.NewEncoder(w).Encode(mapToTransactionResp(t))
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}

func (h *httpHandler) Void(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errMsg := "error reading request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	if len(body) == 0 {
		errMsg := "missing request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	var req VoidRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		errMsg := "failed to unmarshal request body"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeBadRequest)
		return
	}

	void := &domain.Void{
		RequestID:       req.RequestID,
		AuthorizationID: req.AuthorizationID,
	}

	t, err := h.service.Void(ctx, void)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTransactionNotFound):
			errMsg := "unable to find the transaction with the authorization ID"
			_ = WriteError(w, errMsg, CodeNotFound)
			return
		case errors.Is(err, domain.ErrUnprocessable):
			_ = WriteError(w, err.Error(), CodeUnprocessable)
			return
		default:
			errMsg := "failed to void transaction in service"
			_ = WriteError(w, errMsg, CodeUnknownFailure)
			return
		}
	}

	w.Header().Add(ContentType, ApplicationJSON)
	err = json.NewEncoder(w).Encode(mapToTransactionResp(t))
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}

func mapToTransactionResp(t *domain.Transaction) Transaction {
	return Transaction{
		ID:              t.ID,
		AuthorizationID: t.AuthorizationID,
		AuthorizedTime:  t.AuthorizationDate(),
		AuthorizedAmount: Amount{
			MinorUnits: t.AuthorizedAmount.MinorUnits,
			Exponent:   t.AuthorizedAmount.Exponent,
			Currency:   t.AuthorizedAmount.Currency,
		},
		CapturedAmount: Amount{
			MinorUnits: t.CapturedAmount.MinorUnits,
			Exponent:   t.CapturedAmount.Exponent,
			Currency:   t.CapturedAmount.Currency,
		},
		RefundedAmount: Amount{
			MinorUnits: t.RefundedAmount.MinorUnits,
			Exponent:   t.RefundedAmount.Exponent,
			Currency:   t.RefundedAmount.Currency,
		},
		IsVoided: t.Voided(),
	}
}
