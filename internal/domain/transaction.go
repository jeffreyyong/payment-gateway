package domain

import (
	"errors"
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

var (
	ErrTransactionNotAuthorized = errors.New("transaction not authorized")
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrUnprocessable            = errors.New("unprocessable")
)

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
	AuthorizedAmount     Amount
	CapturedAmount       Amount
	RefundedAmount       Amount
	PaymentActionSummary []*PaymentAction
}

func (t Transaction) AuthorizationDate() *time.Time {
	for _, pa := range t.PaymentActionSummary {
		if pa.AuthorizationSuccess() {
			return &pa.ProcessedDate
		}
	}
	return nil
}

func (t Transaction) IsRequestIDIdempotent(pat PaymentActionType, requestID uuid.UUID) bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.Type == pat && pa.RequestID == requestID {
			return true
		}
	}
	return false
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

func (t Transaction) Captured() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.CaptureSuccess() {
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

func (t *Transaction) Amounts() {
	var authorized, captured, refunded uint64
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
	currency := t.Amount.Currency
	exponent := t.Amount.Exponent
	t.AuthorizedAmount = Amount{
		MinorUnits: authorized,
		Currency:   currency,
		Exponent:   exponent,
	}

	t.CapturedAmount = Amount{
		MinorUnits: captured,
		Currency:   currency,
		Exponent:   exponent,
	}
	t.RefundedAmount = Amount{
		MinorUnits: refunded,
		Currency:   currency,
		Exponent:   exponent,
	}
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

	if (t.CapturedAmount.MinorUnits + a.MinorUnits) > t.AuthorizedAmount.MinorUnits {
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

	if (t.RefundedAmount.MinorUnits + a.MinorUnits) > t.CapturedAmount.MinorUnits {
		return errors.New("amount to be refunded > captured amount")
	}
	return nil
}

func (t Transaction) ValidateVoid() error {
	if t.Voided() {
		return errors.New("transaction is already voided")
	}

	if t.Captured() {
		return errors.New("transaction is already captured")
	}
	return nil
}
