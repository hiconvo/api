package db

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

type txContextKey string

const key txContextKey = "tx"

func TransactionFromContext(ctx context.Context) (*datastore.Transaction, bool) {
	maybeTx := ctx.Value(key)
	tx, ok := maybeTx.(*datastore.Transaction)
	if ok && tx != nil {
		return tx, ok
	}

	return &datastore.Transaction{}, false
}

func AddTransactionToContext(ctx context.Context) (context.Context, *datastore.Transaction, error) {
	c := Client.getUnderlyingClient()

	tx, err := c.NewTransaction(ctx)
	if err != nil {
		return ctx, tx, fmt.Errorf("db.AddTransactionToContext: %v", err)
	}

	nctx := context.WithValue(ctx, key, tx)

	return nctx, tx, nil
}
