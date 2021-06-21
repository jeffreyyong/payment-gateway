package domain

import (
	"errors"
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

var (
	// ErrTransactionNotFound indicates that the transaction is not found in the db.
	ErrTransactionNotFound = errors.New("transaction not found")
	// ErrUnprocessable indicates that the request is unprocessable, e.g. due to the wrong state of the transaction.
	ErrUnprocessable = errors.New("unprocessable")
)

// Amount is the canonical amount domain.
type Amount struct {
	MinorUnits uint64
	Currency   string
	Exponent   uint8
}

// PaymentSource is the payment source that the client making payment with.
type PaymentSource struct {
	PAN    string
	CVV    string
	Expiry Expiry
}

// Expiry date of the payment source.
type Expiry struct {
	Month int
	Year  int
}

// Transaction is the transaction domain struct.
// It also contains PaymentActionSummary to show all the PaymentAction that has
// happened to the transaction so far.
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

// AuthorizationDate returns the date when the transaction
// has been authorized successfully, else returns nil.
// Assumption is there is only one Authorization per Transaction on creation.
func (t Transaction) AuthorizationDate() *time.Time {
	for _, pa := range t.PaymentActionSummary {
		if pa.AuthorizationSuccess() {
			return &pa.ProcessedDate
		}
	}
	return nil
}

// IsRequestIDIdempotent checks if the requestID is already been used for a particular
// PaymentActionType and normally will trigger a no op for idempotency.
func (t Transaction) IsRequestIDIdempotent(pat PaymentActionType, requestID uuid.UUID) bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.Type == pat && pa.RequestID == requestID {
			return true
		}
	}
	return false
}

// Voided indicates that the transaction has been voided successfully.
func (t Transaction) Voided() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.VoidSuccess() {
			return true
		}
	}
	return false
}

// Refunded indicates that the transaction has been refunded successfully at least once before.
func (t Transaction) Refunded() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.RefundSuccess() {
			return true
		}
	}
	return false
}

// Captured indicates that the transaction has been captured  successfully at least once before.
func (t Transaction) Captured() bool {
	for _, pa := range t.PaymentActionSummary {
		if pa.CaptureSuccess() {
			return true
		}
	}
	return false
}

// Amounts calculates the main amounts e.g. authorized, captured and refunded amounts
// based on the PaymentActionSummary.
// This is normally called after PaymentActionSummary has been populated.
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

// ValidateCapture rejects if a transaction has been Voided or Refunded before,
// checks the currency is the same and
// rejects if the amount the be captured is greater than the authorized amount.
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

// ValidateRefund rejects if a transaction has been Voided before,
// checks the currency is the same and
// rejects if the amount the be refunded is greater than the captured amount.
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

// ValidateVoid rejects if a transaction has been Voided and Captured before.
func (t Transaction) ValidateVoid() error {
	if t.Voided() {
		return errors.New("transaction is already voided")
	}

	if t.Captured() {
		return errors.New("transaction is already captured")
	}
	return nil
}
