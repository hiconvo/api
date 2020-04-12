package db

import (
	"context"
	"net/http"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/utils/bjson"
)

type ctxKey int

const txKey ctxKey = iota

// TransactionFromContext extracts a transaction from the given
// context is one is present.
func TransactionFromContext(ctx context.Context) (Transaction, bool) {
	tx, ok := ctx.Value(txKey).(Transaction)
	return tx, ok
}

// AddTransactionToContext returns a new context with a transaction added.
func AddTransactionToContext(ctx context.Context) (context.Context, Transaction, error) {
	tx, err := DefaultClient.NewTransaction(ctx)
	if err != nil {
		return ctx, tx, errors.E(errors.Op("db.AddTransactionToContext"), err)
	}

	nctx := context.WithValue(ctx, txKey, tx)

	return nctx, tx, nil
}

// WithTransaction is middleware that adds a transaction to the request context.
func WithTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, tx, err := AddTransactionToContext(r.Context())
		if err != nil {
			bjson.HandleError(w, errors.E(
				errors.Op("db.WithTransaction"),
				errors.Str("could not initialize database transaction"),
				err))
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))

		if tx.Pending() {
			tx.Rollback()
		}

		return
	})
}
