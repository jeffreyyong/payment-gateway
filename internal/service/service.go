//go:generate mockgen -destination=./mocks/store_mock.go -package=mocks github.com/jeffreyyong/payment-gateway/internal/service Store

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"
	uuid "github.com/kevinburke/go.uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/domain"
	"github.com/jeffreyyong/payment-gateway/internal/logging"
	"github.com/jeffreyyong/payment-gateway/internal/luhn"
)

// Store is the db interface
type Store interface {
	Exec(ctx context.Context, f func(ctx context.Context) error) error
	ExecInTransaction(ctx context.Context, f func(ctx context.Context) error) error

	CreateTransaction(ctx context.Context, authorization *domain.Authorization, processedDate time.Time) (*domain.Transaction, error)
	GetTransaction(ctx context.Context, authorizationID uuid.UUID) (*domain.Transaction, error)
	CreatePaymentAction(ctx context.Context, transactionID, requestID uuid.UUID, paymentActionType domain.PaymentActionType,
		amount *domain.Amount, processedDate time.Time) error
}

// Service is the service struct.
type Service struct {
	store Store
	clock clockwork.Clock
}

// NewService initialises a new service with the store and some opts.
func NewService(store Store, opts ...Option) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store", errors.New("invalid param"))
	}

	s := &Service{store: store}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Authorize is the service function to authorize a transaction, it does the luhn validation on the credit card PAN
// and subsequently create a transaction with the authorization.
func (s *Service) Authorize(ctx context.Context, authorization *domain.Authorization) (*domain.Transaction, error) {
	const errLogMsg = "unable to authorize transaction"
	ctx = logging.WithFields(ctx,
		zap.Stringer(logging.RequestID, authorization.RequestID),
		zap.Stringer(logging.PaymentAction, domain.PaymentActionTypeAuthorization))

	if err := luhn.Validate(authorization.PaymentSource.PAN); err != nil {
		err = errors.Wrap(domain.ErrUnprocessable, err.Error())
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	transaction, err := s.store.CreateTransaction(ctx, authorization, s.clock.Now())
	if err != nil {
		err = errors.Wrap(err, "unable to create authorization in store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	return transaction, nil
}

// Void retrieves the transaction that is in the DB based on authorizationID, checks idempotent requests and validation
// and CreatePaymentAction of void for that transaction.
func (s *Service) Void(ctx context.Context, void *domain.Void) (*domain.Transaction, error) {
	const errLogMsg = "unable to void transaction"
	ctx = logging.WithFields(ctx,
		zap.Stringer(logging.RequestID, void.RequestID),
		zap.Stringer(logging.AuthorizationID, void.AuthorizationID),
		zap.Stringer(logging.PaymentAction, domain.PaymentActionTypeVoid))

	transaction, err := s.store.GetTransaction(ctx, void.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get transaction from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	if transaction.IsRequestIDIdempotent(domain.PaymentActionTypeVoid, void.RequestID) {
		logging.Print(ctx, "request is idempotent hence no op")
		return transaction, nil
	}

	if err := transaction.ValidateVoid(); err != nil {
		err = errors.Wrap(domain.ErrUnprocessable, err.Error())
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	err = s.store.CreatePaymentAction(ctx, transaction.ID, void.RequestID, domain.PaymentActionTypeVoid, nil, s.clock.Now())
	if err != nil {
		err = errors.Wrap(err, "unable to create void payment action in store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	transaction, err = s.store.GetTransaction(ctx, void.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get voided transaction from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	return transaction, nil
}

// Capture retrieves the transaction that is in the DB based on authorizationID, checks idempotent requests and validation
// and CreatePaymentAction of capture for that transaction.
func (s *Service) Capture(ctx context.Context, capture *domain.Capture) (*domain.Transaction, error) {
	const errLogMsg = "unable to capture payment"
	ctx = logging.WithFields(ctx,
		zap.Stringer(logging.RequestID, capture.RequestID),
		zap.Stringer(logging.AuthorizationID, capture.AuthorizationID),
		zap.Stringer(logging.PaymentAction, domain.PaymentActionTypeCapture))

	transaction, err := s.store.GetTransaction(ctx, capture.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get transaction from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	if transaction.IsRequestIDIdempotent(domain.PaymentActionTypeCapture, capture.RequestID) {
		logging.Print(ctx, "request is idempotent hence no op")
		return transaction, nil
	}

	if err = transaction.ValidateCapture(capture.Amount); err != nil {
		err = errors.Wrap(domain.ErrUnprocessable, err.Error())
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	err = s.store.CreatePaymentAction(ctx, transaction.ID, capture.RequestID, domain.PaymentActionTypeCapture, &capture.Amount, s.clock.Now())
	if err != nil {
		err = errors.Wrap(err, "unable to create capture payment action in store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	transaction, err = s.store.GetTransaction(ctx, capture.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get transaction with capture from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	return transaction, nil
}

// Refund retrieves the transaction that is in the DB based on authorizationID, checks idempotent requests and validation
// and CreatePaymentAction of refund for that transaction.
func (s *Service) Refund(ctx context.Context, refund *domain.Refund) (*domain.Transaction, error) {
	const errLogMsg = "unable to refund payment"
	ctx = logging.WithFields(ctx,
		zap.Stringer(logging.RequestID, refund.RequestID),
		zap.Stringer(logging.AuthorizationID, refund.AuthorizationID),
		zap.Stringer(logging.PaymentAction, domain.PaymentActionTypeRefund))

	transaction, err := s.store.GetTransaction(ctx, refund.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get transaction from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	if transaction.IsRequestIDIdempotent(domain.PaymentActionTypeRefund, refund.RequestID) {
		logging.Print(ctx, "request is idempotent hence no op")
		return transaction, nil
	}

	if err = transaction.ValidateRefund(refund.Amount); err != nil {
		err = errors.Wrap(domain.ErrUnprocessable, err.Error())
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	err = s.store.CreatePaymentAction(ctx, transaction.ID, refund.RequestID, domain.PaymentActionTypeRefund, &refund.Amount, s.clock.Now())
	if err != nil {
		err = errors.Wrap(err, "unable to create refund payment action in store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	transaction, err = s.store.GetTransaction(ctx, refund.AuthorizationID)
	if err != nil {
		err = errors.Wrap(err, "unable to get transaction with capture from store")
		logging.Error(ctx, errLogMsg, zap.Error(err))
		return nil, err
	}

	return transaction, nil
}
