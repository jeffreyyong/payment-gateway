package domain

import (
	"errors"
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

var (
	ErrTransactionNotAuthorized = errors.New("transaction not authorized")
	ErrTransactionNotFound      = errors.New("transaction not found")
)

type PaymentActionType string
type PaymentActionStatus string
type TransactionStatus string

func (p PaymentActionType) String() string {
	return string(p)
}

const (
	PaymentActionTypeAuthorization PaymentActionType = "authorization"
	PaymentActionTypeVoid          PaymentActionType = "void"
	PaymentActionTypeCapture       PaymentActionType = "capture"
	PaymentActionTypeRefund        PaymentActionType = "refund"
)

const (
	TransactionStatusAuthorized TransactionStatus = "authorized"
	TransactionStatusVoided     TransactionStatus = "voided"
	TransactionStatusCaptured   TransactionStatus = "captured"
	TransactionStatusRefunded   TransactionStatus = "refunded"
)

const (
	PaymentActionStatusSuccess PaymentActionStatus = "success"
	PaymentActionStatusFailed  PaymentActionStatus = "failed"
)

type Authorization struct {
	RequestID     uuid.UUID
	PaymentSource PaymentSource
	Amount        Amount
}

type Capture struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
	Amount          Amount
}

type Refund struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
	Amount          Amount
}

type Void struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
}

type Amount struct {
	MinorUnits uint64
	Currency   string
	// TODO: pull out the auto exponent generator package
	Exponent uint8
}

type PaymentSource struct {
	PAN    string
	CVV    string
	Expiry Expiry
}

type Expiry struct {
	Month int
	Year  int
}

type Transaction struct {
	ID                   uuid.UUID
	RequestID            uuid.UUID
	AuthorizationID      uuid.UUID
	PaymentSource        PaymentSource
	Amount               Amount
	PaymentActionSummary []*PaymentAction
}

type PaymentAction struct {
	Type          PaymentActionType
	Status        PaymentActionStatus
	ProcessedDate time.Time
	Amount        *Amount
	RequestID     uuid.UUID
}
