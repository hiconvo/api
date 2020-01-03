package db

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/utils/bjson"
)

type txContextKey string

const key txContextKey = "tx"

// TransactionFromContext extracts a transaction from the given
// context is one is present.
func TransactionFromContext(ctx context.Context) (*datastore.Transaction, bool) {
	maybeTx := ctx.Value(key)
	tx, ok := maybeTx.(*datastore.Transaction)
	if ok && tx != nil {
		return tx, ok
	}

	return &datastore.Transaction{}, false
}

// AddTransactionToContext returns a new context with a transaction added.
func AddTransactionToContext(ctx context.Context) (context.Context, *datastore.Transaction, error) {
	c := Client.getUnderlyingClient()

	tx, err := c.NewTransaction(ctx)
	if err != nil {
		return ctx, tx, fmt.Errorf("db.AddTransactionToContext: %v", err)
	}

	nctx := context.WithValue(ctx, key, tx)

	return nctx, tx, nil
}

// WithTransaction is middleware that adds a transaction to the request context.
func WithTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		octx := r.Context()
		nctx, _, err := AddTransactionToContext(octx)
		if err != nil {
			bjson.WriteJSON(w, map[string]string{
				"message": "Could not initialize database transaction",
			}, http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(nctx))
		return
	})
}
