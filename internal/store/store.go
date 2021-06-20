package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/jeffreyyong/payment-gateway/internal/logging"
)

// database errors
var (
	ErrConnect            = errors.New("database connection failed")
	ErrPing               = errors.New("database ping failed")
	ErrDriver             = errors.New("database migration driver creation failed")
	ErrReadMigration      = errors.New("database migration reading files failed")
	ErrMigration          = errors.New("database migration failed")
	ErrMissingTransaction = errors.New("database transaction not provided")
)

const (
	postgresDriver = "postgres"
	// migrations table name (payment_gateway_schema_migrations)
	postgresMigrationsTable = "payment_gateway"
	// postgres connection options
	maxOpenConnections = 50
	// must be <= maxOpenConnections
	maxIdleConnections    = 20
	maxConnectionLifetime = time.Second * 1800
)

// Store is a database client wrapper
type Store struct {
	*sql.DB
	ready         bool
	readinessLock sync.RWMutex
}

type connKey struct{}

// New creates the postgres database connection instance
func New(address string) (*Store, error) {
	db, err := sql.Open(postgresDriver, address)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrConnect, err)
	}

	// test database connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrPing, err)
	}

	db.SetMaxIdleConns(maxIdleConnections)
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetConnMaxLifetime(maxConnectionLifetime)

	s := &Store{DB: db}

	return s, nil
}

// Migrate makes sure database migrations are up to date
func (s *Store) Migrate(path string) error {
	// create migration driver
	driver, err := postgres.WithInstance(s.DB, &postgres.Config{
		MigrationsTable: fmt.Sprintf("%s_%s", postgresMigrationsTable, postgres.DefaultMigrationsTable),
	})
	if err != nil {
		return fmt.Errorf("%s: %w", ErrDriver, err)
	}

	// read migration files
	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", path), postgresDriver, driver)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrReadMigration, err)
	}

	// perform database migration
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("%s: %w", ErrMigration, err)
	} else if err == migrate.ErrNoChange {
		v, _, _ := m.Version()
		logging.Print(context.Background(), "postgres migrations up to date", zap.Uint("version", v))
	} else if err == nil {
		v, _, _ := m.Version()
		logging.Print(context.Background(), "postgres database updated", zap.Uint("version", v))
	}

	// update readiness state inside lock
	s.readinessLock.Lock()
	defer s.readinessLock.Unlock()
	s.ready = true
	return nil
}

// Ready checks if the database is ready
func (s *Store) Ready(ctx context.Context) error {
	var ready bool
	s.readinessLock.RLock()
	ready = s.ready
	s.readinessLock.RUnlock()

	if !ready {
		return fmt.Errorf("database not ready")
	}
	return s.DB.PingContext(ctx)
}

func (s *Store) ExecInTransaction(ctx context.Context, f func(context.Context) error) error {
	txn, err := s.DB.Begin()
	if err != nil {
		return err
	}
	if err := f(context.WithValue(ctx, connKey{}, txn)); err != nil {
		if err := txn.Rollback(); err != nil {
			return err
		}
		return err
	}
	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) Exec(ctx context.Context, f func(ctx context.Context) error) error {
	return f(context.WithValue(ctx, connKey{}, s.DB))
}
