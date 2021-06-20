// +build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	uuid "github.com/kevinburke/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jeffreyyong/payment-gateway/internal/domain"
)

var (
	somePAN                = gofakeit.CreditCardNumber(nil)
	someCVV                = gofakeit.CreditCardCvv()
	authorizationRequestID = uuid.NewV4()
	authorization          = &domain.Authorization{
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
			MinorUnits: 10000,
			Exponent:   2,
			Currency:   "GBP",
		},
	}
)

func Test_CreateTransaction_Success(t *testing.T) {
	t.Cleanup(truncateTables)

	wantTransaction := &domain.Transaction{
		RequestID:     authorizationRequestID,
		PaymentSource: authorization.PaymentSource,
		Amount:        authorization.Amount,
		PaymentActionSummary: []*domain.PaymentAction{
			{
				Type:          domain.PaymentActionTypeAuthorization,
				Status:        domain.PaymentActionStatusSuccess,
				ProcessedDate: someFakeDate,
				Amount:        &authorization.Amount,
				RequestID:     authorization.RequestID,
			},
		},
	}

	testCases := []struct {
		description     string
		authorization   *domain.Authorization
		wantTransaction *domain.Transaction
	}{
		{
			"inserts into transaction, card and transaction action",
			authorization,
			wantTransaction,
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.description, func(t *testing.T) {
			gotTransaction, err := s.CreateTransaction(context.Background(), tc.authorization, someFakeDate)
			require.NoError(t, err)
			require.False(t, gotTransaction.AuthorizationID == uuid.Nil)
			require.False(t, gotTransaction.ID == uuid.Nil)
			require.Equal(t, wantTransaction.RequestID, gotTransaction.RequestID)
			require.Equal(t, wantTransaction.PaymentActionSummary, gotTransaction.PaymentActionSummary)
			require.Equal(t, wantTransaction.Amount, gotTransaction.Amount)
			assert.Equal(t, wantTransaction.Amount, gotTransaction.Amount)
		})
	}
}

func Test_GetTransaction_Success(t *testing.T) {
	t.Cleanup(truncateTables)

	ctx := context.Background()
	createdTransaction, err := s.CreateTransaction(ctx, authorization, someFakeDate)
	require.NoError(t, err)

	testCases := []struct {
		description     string
		authorizationID uuid.UUID
		wantTransaction *domain.Transaction
	}{
		{
			"get transaction by authorization ID",
			createdTransaction.AuthorizationID,
			&domain.Transaction{
				ID:              createdTransaction.ID,
				RequestID:       createdTransaction.RequestID,
				AuthorizationID: createdTransaction.AuthorizationID,
				Amount:          createdTransaction.Amount,
				PaymentActionSummary: []*domain.PaymentAction{
					{
						Type:          domain.PaymentActionTypeAuthorization,
						Status:        domain.PaymentActionStatusSuccess,
						ProcessedDate: someFakeDate,
						Amount:        &createdTransaction.Amount,
						RequestID:     createdTransaction.RequestID,
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.description, func(t *testing.T) {
			gotTransaction, err := s.GetTransaction(ctx, tc.authorizationID)
			require.NoError(t, err)
			require.False(t, gotTransaction.AuthorizationID == uuid.Nil)
			require.False(t, gotTransaction.ID == uuid.Nil)
			require.Equal(t, tc.wantTransaction.RequestID.String(), gotTransaction.RequestID.String())
			require.Equal(t, tc.wantTransaction.PaymentSource, gotTransaction.PaymentSource)
			require.Equal(t, tc.wantTransaction.Amount, gotTransaction.Amount)
			require.Equal(t, tc.wantTransaction.Amount, gotTransaction.Amount)
			assert.Equal(t, tc.wantTransaction.PaymentActionSummary, gotTransaction.PaymentActionSummary)
		})
	}
}

func Test_CreatePaymentAction_Success(t *testing.T) {
	t.Cleanup(truncateTables)

	var (
		voidRequestID = uuid.NewV4()
		voidFakeDate  = someFakeDate.Add(1 * time.Hour)
	)

	ctx := context.Background()
	createdTransaction, err := s.CreateTransaction(ctx, authorization, someFakeDate)
	require.NoError(t, err)

	testCases := []struct {
		description       string
		authorizationID   uuid.UUID
		paymentActionType domain.PaymentActionType
		wantTransaction   *domain.Transaction
	}{
		{
			"create void payment action",
			createdTransaction.AuthorizationID,
			domain.PaymentActionTypeVoid,
			&domain.Transaction{
				ID:              createdTransaction.ID,
				RequestID:       createdTransaction.RequestID,
				AuthorizationID: createdTransaction.AuthorizationID,
				Amount:          createdTransaction.Amount,
				PaymentActionSummary: []*domain.PaymentAction{
					{
						Type:          domain.PaymentActionTypeAuthorization,
						Status:        domain.PaymentActionStatusSuccess,
						ProcessedDate: someFakeDate,
						Amount:        &createdTransaction.Amount,
						RequestID:     createdTransaction.RequestID,
					},
					{
						Type:          domain.PaymentActionTypeVoid,
						Status:        domain.PaymentActionStatusSuccess,
						ProcessedDate: voidFakeDate,
						Amount:        nil,
						RequestID:     voidRequestID,
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.description, func(t *testing.T) {
			err := s.CreatePaymentAction(ctx, createdTransaction.ID, voidRequestID, domain.PaymentActionTypeVoid, nil, voidFakeDate)
			require.NoError(t, err)

			gotTransaction, err := s.GetTransaction(ctx, tc.authorizationID)
			require.NoError(t, err)
			require.False(t, gotTransaction.AuthorizationID == uuid.Nil)
			require.False(t, gotTransaction.ID == uuid.Nil)
			require.Equal(t, tc.wantTransaction.RequestID.String(), gotTransaction.RequestID.String())
			require.Equal(t, tc.wantTransaction.PaymentSource, gotTransaction.PaymentSource)
			require.Equal(t, tc.wantTransaction.Amount, gotTransaction.Amount)
			require.Equal(t, tc.wantTransaction.Amount, gotTransaction.Amount)
			assert.Equal(t, tc.wantTransaction.PaymentActionSummary, gotTransaction.PaymentActionSummary)
		})
	}
}
