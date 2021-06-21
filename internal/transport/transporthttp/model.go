package transporthttp

import (
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

// AuthorizeRequest to unmarshal authorization request into
type AuthorizeRequest struct {
	PaymentSource PaymentSource `json:"payment_source"`
	Amount        Amount        `json:"amount"`
	RequestID     uuid.UUID     `json:"request_id"`
	Description   string        `json:"description"`
}

// CaptureRequest to unmarshal capture request into
type CaptureRequest struct {
	AuthorizationID uuid.UUID `json:"authorization_id"`
	RequestID       uuid.UUID `json:"request_id"`
	Amount          Amount    `json:"amount"`
}

// RefundRequest to unmarshal refund request into
type RefundRequest struct {
	AuthorizationID uuid.UUID `json:"authorization_id"`
	RequestID       uuid.UUID `json:"request_id"`
	Amount          Amount    `json:"amount"`
}

// VoidRequest to unmarshal void request into
type VoidRequest struct {
	AuthorizationID uuid.UUID `json:"authorization_id"`
	RequestID       uuid.UUID `json:"request_id"`
}

// PaymentSource request
type PaymentSource struct {
	PAN         string `json:"pan"`
	CVV         string `json:"cvv"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
}

// Amount request
type Amount struct {
	MinorUnits uint64 `json:"minor_units"`
	Exponent   uint8  `json:"exponent"`
	Currency   string `json:"currency"`
}

// Transaction response
type Transaction struct {
	ID               uuid.UUID  `json:"id"`
	AuthorizationID  uuid.UUID  `json:"authorization_id"`
	AuthorizedTime   *time.Time `json:"authorization_date,omitempty"`
	AuthorizedAmount Amount     `json:"authorized_amount"`
	CapturedAmount   Amount     `json:"captured_amount"`
	RefundedAmount   Amount     `json:"refunded_amount"`
	IsVoided         bool       `json:"is_voided"`
}
