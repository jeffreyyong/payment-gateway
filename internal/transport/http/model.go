package http

import (
	"strconv"
	"strings"
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

const (
	DateOfBirthDateFormat = "2006-01-02"
)

// Do validation on http domain
type AuthorizeRequest struct {
	PaymentSource PaymentSource `json:"payment_source"`
	Amount        Amount        `json:"amount"`
	RequestID     uuid.UUID     `json:"request_id"`
	Description   string        `json:"description"`
	Recipient     *Recipient    `json:"recipient,omitempty"`
}

type Recipient struct {
	DateOfBirth DateOfBirth `json:"dob"`
	Postcode    string      `json:"postcode"` // The first part of the UK postcode for example W1T 4TJ would be W1T
	LastName    string      `json:"last_name"`
}

type DateOfBirth struct {
	time.Time
}

// UnmarshalJSON allows us unmarshal time.Time from a custom time format
func (d *DateOfBirth) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "" {
		d.Time = time.Time{}
		return
	}
	d.Time, err = time.Parse(DateOfBirthDateFormat, s)
	return
}

// MarshalJSON will serialise time into a particular format
func (d DateOfBirth) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte(``), nil
	}
	return []byte(strconv.Quote(d.Time.Format(DateOfBirthDateFormat))), nil
}

type PaymentSource struct {
	PAN         string `json:"pan"`
	CVV         string `json:"cvv"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
}

type Amount struct {
	MinorUnits uint64 `json:"minor_units"`
	Exponent   uint8  `json:"exponent"`
	Currency   string `json:"currency"`
}

type Transaction struct {
	ID               uuid.UUID  `json:"id"`
	AuthorizationID  uuid.UUID  `json:"authorization_id"`
	AuthorizedTime   *time.Time `json:"authorization_date,omitempty"`
	AuthorizedAmount Amount     `json:"authorized_amount"`
	CapturedAmount   Amount     `json:"captured_amount"`
	RefundedAmount   Amount     `json:"refunded_amount"`
	IsVoided         bool       `json:"is_voided"`
}
