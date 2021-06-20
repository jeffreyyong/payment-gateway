package main

import (
	"context"
	"os"
	"path"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/app"
	"github.com/jeffreyyong/payment-gateway/internal/app/listeners/httplistener"
	"github.com/jeffreyyong/payment-gateway/internal/config"
	"github.com/jeffreyyong/payment-gateway/internal/logging"
	"github.com/jeffreyyong/payment-gateway/internal/service"
	"github.com/jeffreyyong/payment-gateway/internal/store"
	transporthttp "github.com/jeffreyyong/payment-gateway/internal/transport/http"
)

const (
	serviceName = "payment-gateway"
)

func main() {
	if err := app.Run(serviceName, setup); err != nil {
		logging.Error(context.Background(), "failed to start service",
			zap.String("service", serviceName),
			zap.Error(err),
		)
		panic(err)
	}
}

func setup(ctx context.Context, s *app.Service) ([]app.Listener, context.Context, error) {
	s.OnShutdown(func() {
		logging.Print(ctx, "shutdown",
			zap.String("service", serviceName),
		)
	})

	cfg, err := config.Load()
	if err != nil {
		logging.Error(ctx, "loading_config", zap.Error(err))
		return nil, ctx, err
	}

	store, err := store.New(cfg.PostgresDSN)
	if err != nil {
		logging.Error(ctx, "initialising store", zap.Error(err))
		return nil, ctx, errors.Wrap(err, "initialising store")
	}
	migrationPath, err := migrationPath()
	if err != nil {
		logging.Error(ctx, "unable to get migration path", zap.Error(err))
		return nil, ctx, errors.Wrap(err, "unable to get migration path")
	}
	if err = store.Migrate(migrationPath); err != nil {
		logging.Error(ctx, "unable to migrate")
		return nil, ctx, errors.Wrap(err, "unable to migrate repository")
	}

	svc, err := service.NewService(store)

	if err != nil {
		logging.Error(ctx, "creating_service", zap.Error(err))
		return nil, ctx, err
	}

	h, err := transporthttp.NewHTTPHandler(svc)
	if err != nil {
		logging.Error(ctx, "creating_http_handler", zap.Error(err))
		return nil, ctx, err
	}

	return []app.Listener{httplistener.New(h)}, ctx, nil
}

const (
	defaultMigrationPath = "/migrations"
)

func migrationPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return path.Join(wd, defaultMigrationPath), nil
}
