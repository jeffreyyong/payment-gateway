package transporthttp

import (
	"github.com/gorilla/mux"

	"net/http"
)

const (
	authorizationHeaderKey = "Authorization"
)

type MiddlewareFunc func(c *httpHandler) error

func WithAuth(privilegedTokens map[string]string) MiddlewareFunc {
	return func(h *httpHandler) error {
		h.middlewareFuncs = []mux.MiddlewareFunc{NewAuthorizationMiddleware(privilegedTokens)}
		return nil
	}
}

type HTTPAuthorizeRequest struct {
	next             http.Handler
	privilegedTokens map[string]string
}

func NewAuthorizationMiddleware(privilegedTokens map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &HTTPAuthorizeRequest{
			next:             next,
			privilegedTokens: privilegedTokens,
		}
	}
}

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
