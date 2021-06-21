package domain

import (
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

// PaymentActionType is the type of payment action.
type PaymentActionType string

// PaymentActionStatus is the status of the payment action.
type PaymentActionStatus string

// String() returns the string form and makes PaymentActionType to be a stringer.
func (p PaymentActionType) String() string {
	return string(p)
}

const (
	// PaymentActionTypeAuthorization is type authorization.
	PaymentActionTypeAuthorization PaymentActionType = "authorization"
	// PaymentActionTypeVoid is type void.
	PaymentActionTypeVoid PaymentActionType = "void"
	// PaymentActionTypeCapture is type capture.
	PaymentActionTypeCapture PaymentActionType = "capture"
	// PaymentActionTypeRefund is type refund.
	PaymentActionTypeRefund PaymentActionType = "refund"
)

const (
	// PaymentActionStatusSuccess indicates that the payment action has succeeded.
	PaymentActionStatusSuccess PaymentActionStatus = "success"
	// PaymentActionStatusFailed indicates that the payment action has failed.
	PaymentActionStatusFailed PaymentActionStatus = "failed"
)

// Authorization is the domain for making authorization request.
type Authorization struct {
	RequestID     uuid.UUID
	PaymentSource PaymentSource
	Amount        Amount
}

// Capture is the domain for making capture request.
type Capture struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
	Amount          Amount
}

// Refund is the domain for making refund request.
type Refund struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
	Amount          Amount
}

// Void is the domain for making void request.
// It does not take in Amount as void is for the whole transaction.
type Void struct {
	RequestID       uuid.UUID
	AuthorizationID uuid.UUID
}

// PaymentAction is the payment action domain.
type PaymentAction struct {
	Type          PaymentActionType
	Status        PaymentActionStatus
	ProcessedDate time.Time
	Amount        *Amount
	RequestID     uuid.UUID
}

// AuthorizationSuccess means the authorization has succeeded.
func (p PaymentAction) AuthorizationSuccess() bool {
	return p.Type == PaymentActionTypeAuthorization && p.Status == PaymentActionStatusSuccess
}

// VoidSuccess means the void has succeeded.
func (p PaymentAction) VoidSuccess() bool {
	return p.Type == PaymentActionTypeVoid && p.Status == PaymentActionStatusSuccess
}

// CaptureSuccess means the capture has succeeded.
func (p PaymentAction) CaptureSuccess() bool {
	return p.Type == PaymentActionTypeCapture && p.Status == PaymentActionStatusSuccess
}

// RefundSuccess means the refund has succeeded.
func (p PaymentAction) RefundSuccess() bool {
	return p.Type == PaymentActionTypeRefund && p.Status == PaymentActionStatusSuccess
}
