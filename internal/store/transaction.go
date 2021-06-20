package store

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	uuid "github.com/kevinburke/go.uuid"
	"github.com/pkg/errors"

	"github.com/jeffreyyong/payment-gateway/internal/domain"
)

// TODO: think about idempotency?

// CreateTransaction creates the first ever transaction, it will populate the transaction table, card table and
// the payment_action table with Authorization type
func (s *Store) CreateTransaction(ctx context.Context, authorization *domain.Authorization, processedDate time.Time) (*domain.Transaction, error) {
	var (
		tx                      *sql.Tx
		stmtCardInsert          *sql.Stmt
		stmtTransactionInsert   *sql.Stmt
		stmtPaymentActionInsert *sql.Stmt
		err                     error

		cardID          uuid.UUID
		transactionID   uuid.UUID
		paymentActionID uuid.UUID
	)

	tx, err = s.Begin()
	if err != nil {
		return nil, errors.Wrap(err, "begin transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// insert card
	stmtCardInsert, err = tx.PrepareContext(ctx, `
		insert into card (pan, cvv, expiry_month, expiry_year, created_date, updated_date)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (pan)
		do update set pan = excluded.pan
		returning id
	`)
	if err != nil {
		return nil, errors.Wrap(err, "prepare insert card statement")
	}
	defer stmtCardInsert.Close()

	ps := authorization.PaymentSource
	if err = stmtCardInsert.
		QueryRowContext(ctx, ps.PAN, ps.CVV, strconv.Itoa(ps.Expiry.Month), strconv.Itoa(ps.Expiry.Year), processedDate, processedDate).
		Scan(&cardID); err != nil {
		return nil, errors.Wrap(err, "execute insert card statement")
	}

	authorizationID := uuid.NewV4()

	// insert transaction
	stmtTransactionInsert, err = tx.PrepareContext(ctx, `
		insert into transaction (card_id, authorization_id, request_id, amount, currency, created_date, updated_date)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (request_id)
		do update set request_id = excluded.request_id
		returning id
	`)
	if err != nil {
		return nil, errors.Wrap(err, "prepare insert authorization statement")
	}
	defer stmtTransactionInsert.Close()

	if err = stmtTransactionInsert.
		QueryRowContext(ctx, cardID, authorizationID, authorization.RequestID, authorization.Amount.MinorUnits, authorization.Amount.Currency, processedDate, processedDate).
		Scan(&transactionID); err != nil {
		return nil, errors.Wrap(err, "execute insert authorization statement")
	}

	// insert payment action
	stmtPaymentActionInsert, err = tx.PrepareContext(ctx, `
		insert into payment_action (id, type, status, amount, currency, request_id, transaction_id, created_date, updated_date)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		on conflict (request_id)
		do update set request_id = excluded.request_id
		returning id
	`)
	if err != nil {
		return nil, errors.Wrap(err, "prepare insert payment action statement")
	}
	defer stmtPaymentActionInsert.Close()

	if err = stmtPaymentActionInsert.
		QueryRowContext(ctx, authorizationID, domain.PaymentActionTypeAuthorization,
			domain.PaymentActionStatusSuccess, authorization.Amount.MinorUnits, authorization.Amount.Currency,
			authorization.RequestID, transactionID, processedDate, processedDate).
		Scan(&paymentActionID); err != nil {
		return nil, errors.Wrap(err, "execute insert payment action statement")
	}

	paymentAction := &domain.PaymentAction{
		Type:          domain.PaymentActionTypeAuthorization,
		Status:        domain.PaymentActionStatusSuccess,
		ProcessedDate: processedDate,
		Amount:        &authorization.Amount,
		RequestID:     authorization.RequestID,
	}

	transactionRes := &domain.Transaction{
		ID:              transactionID,
		RequestID:       authorization.RequestID,
		AuthorizationID: authorizationID,
		Amount:          authorization.Amount,
		PaymentActionSummary: []*domain.PaymentAction{
			paymentAction,
		},
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return transactionRes, nil
}

//CreatePaymentAction will create payment action for a particular transaction.
func (s *Store) CreatePaymentAction(ctx context.Context, transactionID, requestID uuid.UUID, paymentActionType domain.PaymentActionType,
	amount *domain.Amount, processedDate time.Time) error {
	var (
		tx                      *sql.Tx
		stmtPaymentActionInsert *sql.Stmt
		err                     error

		paymentActionID uuid.UUID
	)

	tx, err = s.Begin()
	if err != nil {
		return errors.Wrap(err, "begin transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// insert payment action
	stmtPaymentActionInsert, err = tx.PrepareContext(ctx, `
		insert into payment_action (type, status, amount, currency, request_id, transaction_id, created_date, updated_date)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (request_id)
		do update set request_id = excluded.request_id
		returning id
	`)
	if err != nil {
		return errors.Wrap(err, "prepare insert payment action statement")
	}
	defer stmtPaymentActionInsert.Close()

	var minorUnits, currency interface{}
	if amount != nil {
		minorUnits = amount.MinorUnits
		currency = amount.Currency
	}

	if err = stmtPaymentActionInsert.
		QueryRowContext(ctx, paymentActionType,
			domain.PaymentActionStatusSuccess, minorUnits, currency,
			requestID, transactionID, processedDate, processedDate).
		Scan(&paymentActionID); err != nil {
		return errors.Wrap(err, "execute insert payment action statement")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "commit create payment action")
	}

	return nil
}

func (s *Store) GetTransaction(ctx context.Context, authorizationID uuid.UUID) (*domain.Transaction, error) {
	rows, err := s.QueryContext(ctx, `
		select t.id as t_id, t.request_id as t_request_id, t.amount, t.currency, p.id as p_id, p.type, p.status, p.amount, p.currency, p.request_id as p_request_id, p.updated_date
		from transaction t JOIN payment_action p ON t.id = p.transaction_id where t.authorization_id = $1 order by p.created_date;
		`, authorizationID)

	if err != nil {
		return nil, errors.Wrap(err, "get transaction query")
	}
	defer rows.Close()

	paymentActionSummary := make([]*domain.PaymentAction, 0)

	var (
		transactionID              uuid.UUID
		transactionRequestID       uuid.UUID
		transactionAmount          sql.NullInt64
		transactionCurrency        sql.NullString
		paymentActionID            uuid.UUID
		paymentActionType          sql.NullString
		paymentActionStatus        sql.NullString
		paymentActionAmount        sql.NullInt64
		paymentActionCurrency      sql.NullString
		paymentActionRequestID     uuid.UUID
		paymentActionProcessedDate sql.NullTime
	)

	for rows.Next() {
		if err := rows.Scan(&transactionID, &transactionRequestID, &transactionAmount, &transactionCurrency, &paymentActionID,
			&paymentActionType, &paymentActionStatus, &paymentActionAmount, &paymentActionCurrency, &paymentActionRequestID, &paymentActionProcessedDate); err != nil {
			return nil, errors.Wrap(err, "get transaction scanning")
		}
		// TODO: this can be mapped properly
		exponent := 2

		var amount *domain.Amount
		if paymentActionAmount.Valid {
			amount = &domain.Amount{
				MinorUnits: uint64(paymentActionAmount.Int64),
				Currency:   paymentActionCurrency.String,
				Exponent:   uint8(exponent),
			}
		}

		paymentAction := &domain.PaymentAction{
			Type:          domain.PaymentActionType(paymentActionType.String),
			Status:        domain.PaymentActionStatus(paymentActionStatus.String),
			ProcessedDate: paymentActionProcessedDate.Time,
			Amount:        amount,
			RequestID:     paymentActionRequestID,
		}

		paymentActionSummary = append(paymentActionSummary, paymentAction)
	}

	if rows.Err() != nil {
		return nil, errors.Wrap(rows.Err(), "get transaction rows err")
	}

	if len(paymentActionSummary) == 0 {
		return nil, domain.ErrTransactionNotFound
	}

	transaction := &domain.Transaction{
		ID:              transactionID,
		RequestID:       transactionRequestID,
		AuthorizationID: authorizationID,
		Amount: domain.Amount{
			MinorUnits: uint64(transactionAmount.Int64),
			Currency:   transactionCurrency.String,
			Exponent:   2,
		},
		PaymentActionSummary: paymentActionSummary,
	}
	transaction.Amounts()

	return transaction, nil
}
