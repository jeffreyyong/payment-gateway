package context

import "context"

type CtxKey string

const (
	ContextAPI     CtxKey = "api"
	ContextService CtxKey = "service"
)

func WithAPI(ctx context.Context, api string) context.Context {
	return context.WithValue(ctx, ContextAPI, api)
}

func GetAPI(ctx context.Context) string {
	api := ctx.Value(ContextAPI)
	if api != nil {
		return api.(string)
	}
	return ""
}

func WithService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, ContextService, service)
}
