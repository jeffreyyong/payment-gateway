package domain

import (
	"time"

	uuid "github.com/kevinburke/go.uuid"
)

type PaymentActionType string
type PaymentActionStatus string

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
