package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/golang/mock/gomock"
	"github.com/jonboulle/clockwork"
	uuid "github.com/kevinburke/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffreyyong/payment-gateway/internal/domain"
	"github.com/jeffreyyong/payment-gateway/internal/service"
	"github.com/jeffreyyong/payment-gateway/internal/service/mocks"
)

var (
	someDate               = time.Date(2021, 5, 2, 12, 0, 0, 0, time.UTC)
	somePAN                = gofakeit.CreditCardNumber(nil)
	someCVV                = gofakeit.CreditCardCvv()
	authorizationID        = uuid.NewV4()
	transactionID          = uuid.NewV4()
	authorizationRequestID = uuid.NewV4()
	voidRequestID          = uuid.NewV4()
	transactionAmount      = uint64(10000)
	transactionCurrency    = "GBP"
	fullCaptureAmount      = uint64(10000)
	captureRequestID       = uuid.NewV4()
	refundRequestID        = uuid.NewV4()
	fullRefundAmount       = uint64(10000)

	authorization = &domain.Authorization{
		RequestID: authorizationRequestID,
		PaymentSource: domain.PaymentSource{
			PAN: somePAN,
			CVV: someCVV,
			Expiry: domain.Expiry{
				Month: 1,
				Year:  23,
			},
		},
		Amount: domain.Amount{
			MinorUnits: transactionAmount,
			Exponent:   2,
			Currency:   transactionCurrency,
		},
	}

	void = &domain.Void{
		RequestID:       voidRequestID,
		AuthorizationID: authorizationID,
	}

	voidedPaymentAction = &domain.PaymentAction{
		Type:          domain.PaymentActionTypeVoid,
		Status:        domain.PaymentActionStatusSuccess,
		ProcessedDate: someDate.Add(1 * time.Hour),
		RequestID:     voidRequestID,
	}

	capture = &domain.Capture{
		RequestID:       captureRequestID,
		AuthorizationID: authorizationID,
		Amount: domain.Amount{
			MinorUnits: fullCaptureAmount,
			Exponent:   2,
			Currency:   transactionCurrency,
		},
	}

	capturePaymentAction = &domain.PaymentAction{
		Type:          domain.PaymentActionTypeCapture,
		Status:        domain.PaymentActionStatusSuccess,
		ProcessedDate: someDate.Add(1 * time.Hour),
		RequestID:     captureRequestID,
		Amount: &domain.Amount{
			MinorUnits: fullCaptureAmount,
			Exponent:   2,
			Currency:   transactionCurrency,
		},
	}

	refund = &domain.Refund{
		RequestID:       refundRequestID,
		AuthorizationID: authorizationID,
		Amount: domain.Amount{
			MinorUnits: fullCaptureAmount,
			Exponent:   2,
			Currency:   transactionCurrency,
		},
	}

	refundPaymentAction = &domain.PaymentAction{
		Type:          domain.PaymentActionTypeRefund,
		Status:        domain.PaymentActionStatusSuccess,
		ProcessedDate: someDate.Add(2 * time.Hour),
		RequestID:     refundRequestID,
		Amount: &domain.Amount{
			MinorUnits: fullRefundAmount,
			Exponent:   2,
			Currency:   transactionCurrency,
		},
	}

	mockAuthorizedTransaction = domain.Transaction{
		ID:               transactionID,
		RequestID:        authorization.RequestID,
		AuthorizationID:  authorizationID,
		PaymentSource:    authorization.PaymentSource,
		Amount:           authorization.Amount,
		AuthorizedAmount: authorization.Amount,
		PaymentActionSummary: []*domain.PaymentAction{
			{
				Type:          domain.PaymentActionTypeAuthorization,
				Status:        domain.PaymentActionStatusSuccess,
				ProcessedDate: someDate,
				Amount:        &authorization.Amount,
				RequestID:     authorization.RequestID,
			},
		},
	}

	mockVoidedTransaction   = appendPaymentAction(mockAuthorizedTransaction, voidedPaymentAction)
	mockCapturedTransaction = appendPaymentAction(mockAuthorizedTransaction, capturePaymentAction)
	mockRefundedTransaction = appendPaymentAction(mockCapturedTransaction, refundPaymentAction)
)

func TestService_Authorize_Success(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mocks.NewMockStore(ctrl)

	s, err := service.NewService(store, service.WithClock(clockwork.NewFakeClockAt(someDate)))
	require.NoError(t, err)

	store.EXPECT().CreateTransaction(gomock.Any(), authorization, someDate).Return(&mockAuthorizedTransaction, nil)

	transaction, err := s.Authorize(ctx, authorization)
	require.NoError(t, err)
	assert.Equal(t, &mockAuthorizedTransaction, transaction)
}

// TODO: generate test coverage
func TestService_Void_Success(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mocks.NewMockStore(ctrl)

	s, err := service.NewService(store, service.WithClock(clockwork.NewFakeClockAt(someDate)))
	require.NoError(t, err)

	gomock.InOrder(
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockAuthorizedTransaction, nil).Times(1),
		store.EXPECT().CreatePaymentAction(gomock.Any(), mockAuthorizedTransaction.ID, voidRequestID,
			domain.PaymentActionTypeVoid, nil, someDate).Return(nil).Times(1),
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockVoidedTransaction, nil).Times(1),
	)

	transaction, err := s.Void(ctx, void)
	require.NoError(t, err)
	assert.Equal(t, &mockVoidedTransaction, transaction)
}

func TestService_Capture_Success(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mocks.NewMockStore(ctrl)

	s, err := service.NewService(store, service.WithClock(clockwork.NewFakeClockAt(someDate)))
	require.NoError(t, err)

	gomock.InOrder(
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockAuthorizedTransaction, nil).Times(1),
		store.EXPECT().CreatePaymentAction(gomock.Any(), mockAuthorizedTransaction.ID, captureRequestID,
			domain.PaymentActionTypeCapture, &capture.Amount, someDate).Return(nil).Times(1),
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockVoidedTransaction, nil).Times(1),
	)

	transaction, err := s.Capture(ctx, capture)
	require.NoError(t, err)
	assert.Equal(t, &mockVoidedTransaction, transaction)
}

func TestService_Refund_Success(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mocks.NewMockStore(ctrl)

	s, err := service.NewService(store, service.WithClock(clockwork.NewFakeClockAt(someDate)))
	require.NoError(t, err)

	gomock.InOrder(
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockCapturedTransaction, nil).Times(1),
		store.EXPECT().CreatePaymentAction(gomock.Any(), mockAuthorizedTransaction.ID, refundRequestID, domain.PaymentActionTypeRefund, &refund.Amount, someDate).Return(nil).Times(1),
		store.EXPECT().GetTransaction(gomock.Any(), authorizationID).Return(&mockRefundedTransaction, nil).Times(1),
	)

	transaction, err := s.Refund(ctx, refund)
	require.NoError(t, err)
	assert.Equal(t, &mockRefundedTransaction, transaction)
}

func appendPaymentAction(t domain.Transaction, pa *domain.PaymentAction) domain.Transaction {
	t.PaymentActionSummary = append(t.PaymentActionSummary, pa)
	t.Amounts()
	return t
}
