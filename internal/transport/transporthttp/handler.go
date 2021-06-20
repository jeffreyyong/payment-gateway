//go:generate mockgen -package http -self_package github.com/jeffreyyong/payment-gateway/internal/transport/http -destination handler_mock.go github.com/jeffreyyong/payment-gateway/internal/transport/http Service

package transporthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/jeffreyyong/payment-gateway/internal/logging"
	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/app/listeners/httplistener"
	"github.com/jeffreyyong/payment-gateway/internal/domain"
)

const (
	EndpointAuthorize = "/authorize"

	ContentType     = "Content-Type"
	ApplicationJSON = "application/json"
)

// Service represents an interface for a service layer allowing HTTP routing logic and business logic to be separated
type Service interface {
	Authorize(ctx context.Context, authorization *domain.Authorization) (*domain.Transaction, error)
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
}

func (h *httpHandler) Capture(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TODO: if request ID is the same, tell the client is no op
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
		Recipient: domain.Recipient{
			Postcode: req.Recipient.Postcode,
			LastName: req.Recipient.LastName,
		},
	}

	t, err := h.service.Authorize(ctx, authorization)
	if err != nil {
		// TODO: do more mapping of errors like not found
		errMsg := "failed to authorize transaction in service"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}

	authorizationDate, err := t.AuthorizationDate()
	if err != nil {
		errMsg := "invalid transaction with no authorization date"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}

	transactionRes := Transaction{
		ID:              t.ID,
		AuthorizationID: t.AuthorizationID,
		AuthorizedTime:  &authorizationDate,
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

	w.Header().Add(ContentType, ApplicationJSON)
	err = json.NewEncoder(w).Encode(transactionRes)
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}
