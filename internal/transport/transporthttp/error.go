package transporthttp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

const (
	CodeNone = "none"

	CodeUnauthorized       = "unauthorized"
	CodeTokenExpired       = "auth_token_expired"
	CodeForbidden          = "permission_denied"
	CodeNotFound           = "not_found"
	CodeOperationNotFound  = "operation_not_found"
	CodeBadResponse        = "bad_response"
	CodeUnknownFailure     = "unknown_failure"
	CodeConflict           = "conflict"
	CodeBadRequest         = "bad_request"
	CodePreconditionFailed = "failed_precondition"
)

var (
	codeMap = map[string]int{
		CodeNone:               http.StatusBadGateway,
		CodeUnauthorized:       http.StatusUnauthorized,
		CodeTokenExpired:       http.StatusUnauthorized,
		CodeForbidden:          http.StatusForbidden,
		CodeNotFound:           http.StatusNotFound,
		CodeOperationNotFound:  http.StatusNotFound,
		CodeBadResponse:        http.StatusBadGateway,
		CodeUnknownFailure:     http.StatusInternalServerError,
		CodeBadRequest:         http.StatusBadRequest,
		CodeConflict:           http.StatusConflict,
		CodePreconditionFailed: http.StatusPreconditionFailed,
	}
)

//WriteError writes a json response and pre-registered http status error
// always writes response even when producing an error
func WriteError(w http.ResponseWriter, message, code string) error {
	serverError := ServerError{
		Code:    code,
		Message: message,
	}
	var err error
	w.Header().Set("Content-Type", "application/json")
	sc, ok := codeMap[serverError.Code]
	if !ok {
		err = fmt.Errorf("code not registered %v", serverError)
		sc = codeMap[serverError.Code]
	}
	w.WriteHeader(sc)

	enc := json.NewEncoder(w)

	if encErr := enc.Encode(serverError); encErr != nil {
		// allow encoding error to override the unregistered code error
		err = encErr
	}

	return err
}
