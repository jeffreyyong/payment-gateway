package transporthttp

import (
	"github.com/gorilla/mux"

	"net/http"
)

const (
	authorizationHeaderKey = "Authorization"
)

// MiddlewareFunc type
type MiddlewareFunc func(c *httpHandler) error

// WithAuth is a function configuration for authorization
func WithAuth(privilegedTokens map[string]string) MiddlewareFunc {
	return func(h *httpHandler) error {
		h.middlewareFuncs = []mux.MiddlewareFunc{NewAuthorizationMiddleware(privilegedTokens)}
		return nil
	}
}

// HTTPAuthorizeRequest is the type to handles authorization of request
type HTTPAuthorizeRequest struct {
	next             http.Handler
	privilegedTokens map[string]string
}

// NewAuthorizationMiddleware initialises a http.Handler implementation of authorization given the privileged tokens.
func NewAuthorizationMiddleware(privilegedTokens map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &HTTPAuthorizeRequest{
			next:             next,
			privilegedTokens: privilegedTokens,
		}
	}
}

// ServeHTTP chains the middlewares and does the corresponding authorization for the incoming request.
func (a HTTPAuthorizeRequest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get(authorizationHeaderKey)
	if apiKey == "" {
		_ = WriteError(w, "Authorization missing", CodeUnauthorized)
		return
	}

	if _, ok := a.privilegedTokens[apiKey]; !ok {
		_ = WriteError(w, "invalid token", CodeForbidden)
		return
	}
	ctx := r.Context()
	a.next.ServeHTTP(w, r.WithContext(ctx))
}
