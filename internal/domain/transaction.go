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
	Exponent   uint8
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

func (p PaymentAction) AuthorizationSuccess() bool {
	return p.Type == PaymentActionTypeAuthorization && p.Status == PaymentActionStatusSuccess
}

func (p PaymentAction) VoidSuccess() bool {
	return p.Type == PaymentActionTypeVoid && p.Status == PaymentActionStatusSuccess
}

func (p PaymentAction) CaptureSuccess() bool {
	return p.Type == PaymentActionTypeCapture && p.Status == PaymentActionStatusSuccess
}

func (p PaymentAction) RefundSuccess() bool {
	return p.Type == PaymentActionTypeRefund && p.Status == PaymentActionStatusSuccess
}

func (t Transaction) Voided() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.VoidSuccess() {
			return true
		}
	}
	return false
}

func (t Transaction) Refunded() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.RefundSuccess() {
			return true
		}
	}
	return false
}

func (t Transaction) HasPaymentAction(pat PaymentActionType) bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.Type == pat && pa.Status == PaymentActionStatusSuccess {
			return true
		}
	}
	return false
}

func (t Transaction) amounts() (authorized, captured, refunded uint64) {
	for _, pa := range t.PaymentActionSummary {
		if pa.AuthorizationSuccess() {
			authorized = pa.Amount.MinorUnits
		}

		if pa.CaptureSuccess() {
			captured += pa.Amount.MinorUnits
		}

		if pa.RefundSuccess() {
			refunded += pa.Amount.MinorUnits
		}
	}
	return
}

func (t Transaction) ValidateCapture(a Amount) error {
	if t.Voided() {
		return errors.New("transaction is already voided")
	}

	if t.Refunded() {
		return errors.New("transaction is already refunded")
	}

	if t.Amount.Currency != a.Currency {
		return errors.New("currency is different")
	}

	authorizedAmount, capturedAmount, _ := t.amounts()
	if (capturedAmount + a.MinorUnits) > authorizedAmount {
		return errors.New("amount to be captured > authorized amount")
	}
	return nil
}

func (t Transaction) ValidateRefund(a Amount) error {
	if t.Voided() {
		return errors.New("transaction is already voided")
	}

	if t.Amount.Currency != a.Currency {
		return errors.New("currency is different")
	}

	_, capturedAmount, refundedAmount := t.amounts()
	if (refundedAmount + a.MinorUnits) > capturedAmount {
		return errors.New("amount to be refunded > captured amount")
	}
	return nil
}
