package db

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hiconvo/api/utils/bjson"
)

type txContextKey string

const key txContextKey = "tx"

// TransactionFromContext extracts a transaction from the given
// context is one is present.
func TransactionFromContext(ctx context.Context) (Transaction, bool) {
	maybeTx := ctx.Value(key)
	tx, ok := maybeTx.(Transaction)
	if ok && tx != nil {
		return tx, ok
	}

	return &wrappedTransactionImpl{}, false
}

// AddTransactionToContext returns a new context with a transaction added.
func AddTransactionToContext(ctx context.Context) (context.Context, Transaction, error) {
	tx, err := Client.NewTransaction(ctx)
	if err != nil {
		return ctx, tx, fmt.Errorf("db.AddTransactionToContext: %v", err)
	}

	nctx := context.WithValue(ctx, key, tx)

	return nctx, tx, nil
}

// WithTransaction is middleware that adds a transaction to the request context.
func WithTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, tx, err := AddTransactionToContext(r.Context())
		if err != nil {
			bjson.WriteJSON(w, map[string]string{
				"message": "Could not initialize database transaction",
			}, http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))

		if tx.Pending() {
			tx.Rollback()
		}

		return
	})
}
