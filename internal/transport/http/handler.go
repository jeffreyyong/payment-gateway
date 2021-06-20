package http

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
	endpointAuthorize = "/authorize"

	contentType     = "Content-Type"
	applicationJSON = "application/json"
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
	m.HandleFunc(endpointAuthorize, h.Authorize).Methods(http.MethodPost)
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
			DateOfBirth: req.Recipient.DateOfBirth.Time,
			Postcode:    req.Recipient.Postcode,
			LastName:    req.Recipient.LastName,
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
		errMsg := "invalid transaction"
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

	w.Header().Add(contentType, applicationJSON)
	err = json.NewEncoder(w).Encode(transactionRes)
	if err != nil {
		errMsg := "error encoding json response"
		logging.Error(ctx, errMsg, zap.Error(err))
		_ = WriteError(w, errMsg, CodeUnknownFailure)
		return
	}
}

type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

const (
	CodeNone = "none"

	CodeUnauthorized       = "unauthorized"
	CodeTokenExpired       = "auth_token_expired"
	CodeForbidden          = "permission_denied"
	CodeNotFound           = "not_found"
	CodeOperationNotFound  = "operation_not_found"
	CodeBadResponse        = "bad_response"
	CodeUnknownFailure     = "unknown_failure"
	CodeConflict           = "conflict"
	CodeBadRequest         = "bad_request"
	CodePreconditionFailed = "failed_precondition"
)

var (
	codeMap = map[string]int{
		CodeNone:               http.StatusBadGateway,
		CodeUnauthorized:       http.StatusUnauthorized,
		CodeTokenExpired:       http.StatusUnauthorized,
		CodeForbidden:          http.StatusForbidden,
		CodeNotFound:           http.StatusNotFound,
		CodeOperationNotFound:  http.StatusNotFound,
		CodeBadResponse:        http.StatusBadGateway,
		CodeUnknownFailure:     http.StatusInternalServerError,
		CodeBadRequest:         http.StatusBadRequest,
		CodeConflict:           http.StatusConflict,
		CodePreconditionFailed: http.StatusPreconditionFailed,
	}
)

//WriteError writes a json response and pre-registered http status error
// always writes response even when producing an error
func WriteError(w http.ResponseWriter, message, code string) error {
	serverError := ServerError{
		Code:    code,
		Message: message,
	}
	var err error
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	sc, ok := codeMap[serverError.Code]
	if !ok {
		err = fmt.Errorf("code not registered %v", serverError)
		sc = codeMap[serverError.Code]
	}
	w.WriteHeader(sc)

	enc := json.NewEncoder(w)

	if encErr := enc.Encode(serverError); encErr != nil {
		// allow encoding error to override the unregistered code error
		err = encErr
	}

	return err
}
