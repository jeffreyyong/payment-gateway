// +build integration

package store_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/jeffreyyong/payment-gateway/internal/store"
)

const postgresDSN = "postgres://username:password@localhost:5432/db-payment-gateway?sslmode=disable"

var (
	s            *store.Store
	db           *sql.DB
	ctx          = context.Background()
	someFakeDate = time.Date(2021, 5, 2, 12, 0, 0, 0, time.UTC)
)

// TestMain will setup the conn with PostgreSQL.
func TestMain(m *testing.M) {
	var err error
	db, err = sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("creating_postgres_client: %v", err)
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		log.Fatalf("ping_postgres: %v", err)
	}

	s, err = store.New(postgresDSN)
	if err != nil {
		log.Fatalf("initialising store: %v", err)
	}
	err = s.Migrate("../../migrations")
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func truncateTables() {
	ctx := context.Background()
	if _, err := s.ExecContext(ctx, `truncate table transaction cascade`); err != nil {
		log.Fatalf("truncate table transaction failed: %v", err)
	}
	if _, err := s.ExecContext(ctx, `truncate table payment_action cascade`); err != nil {
		log.Fatalf("truncate table payment_action failed: %v", err)
	}
	if _, err := s.ExecContext(ctx, `truncate table card cascade`); err != nil {
		log.Fatalf("truncate table card failed: %v", err)
	}
}
