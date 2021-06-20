package context

import "context"

type CtxKey string

const (
	ContextAPI     CtxKey = "api"
	ContextEnv     CtxKey = "env"
	ContextService CtxKey = "service"
	ContextVersion CtxKey = "version"
	ContextTraceID CtxKey = "traceID"
	ContextSpanID  CtxKey = "spanID"
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

func GetService(ctx context.Context) string {
	service := ctx.Value(ContextService)
	if service != nil {
		return service.(string)
	}
	return ""
}

func WithEnv(ctx context.Context, env string) context.Context {
	return context.WithValue(ctx, ContextEnv, env)
}

func GetEnv(ctx context.Context) string {
	env := ctx.Value(ContextEnv)
	if env != nil {
		return env.(string)
	}
	return ""
}

func WithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, ContextVersion, version)
}

func GetVersion(ctx context.Context) string {
	version := ctx.Value(ContextVersion)
	if version != nil {
		return version.(string)
	}
	return ""
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ContextTraceID, traceID)
}

func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, ContextSpanID, spanID)
}
