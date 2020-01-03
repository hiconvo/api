package db

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
)

type wrappedClient interface {
	Count(ctx context.Context, q *datastore.Query) (n int, err error)
	Delete(ctx context.Context, key *datastore.Key) error
	DeleteMulti(ctx context.Context, keys []*datastore.Key) (err error)
	Get(ctx context.Context, key *datastore.Key, dst interface{}) (err error)
	GetAll(ctx context.Context, q *datastore.Query, dst interface{}) (keys []*datastore.Key, err error)
	GetMulti(ctx context.Context, keys []*datastore.Key, dst interface{}) (err error)
	Put(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.Key, error)
	PutWithTransaction(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.PendingKey, error)
	PutMulti(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.Key, err error)
	PutMultiWithTransaction(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.PendingKey, err error)
	RunInTransaction(ctx context.Context, f func(tx *datastore.Transaction) error) (*datastore.Commit, error)
	Run(ctx context.Context, q *datastore.Query) *datastore.Iterator

	getUnderlyingClient() *datastore.Client
}

// wrappedClientImpl is a shim around datastore.Client that detects
// if a transaction is available on the current context and uses it
// if there is. Otherwise, it calls the corresponding
// datastore.Client method.
type wrappedClientImpl struct {
	client *datastore.Client
}

func newClient(ctx context.Context, projectID string) wrappedClient {
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		panic(err)
	}

	return &wrappedClientImpl{client: client}
}

func (c *wrappedClientImpl) Count(ctx context.Context, q *datastore.Query) (int, error) {
	return c.client.Count(ctx, q)
}

func (c *wrappedClientImpl) Delete(ctx context.Context, key *datastore.Key) error {
	if tx, ok := TransactionFromContext(ctx); ok {
		err := tx.Delete(key)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	}

	return c.client.Delete(ctx, key)
}

func (c *wrappedClientImpl) DeleteMulti(ctx context.Context, keys []*datastore.Key) error {
	if tx, ok := TransactionFromContext(ctx); ok {
		err := tx.DeleteMulti(keys)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	}

	return c.client.DeleteMulti(ctx, keys)
}

func (c *wrappedClientImpl) Get(ctx context.Context, key *datastore.Key, dst interface{}) error {
	if tx, ok := TransactionFromContext(ctx); ok {
		err := tx.Get(key, dst)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	}

	return c.client.Get(ctx, key, dst)
}

func (c *wrappedClientImpl) GetAll(ctx context.Context, q *datastore.Query, dst interface{}) (keys []*datastore.Key, err error) {
	return c.client.GetAll(ctx, q, dst)
}

func (c *wrappedClientImpl) GetMulti(ctx context.Context, keys []*datastore.Key, dst interface{}) (err error) {
	if tx, ok := TransactionFromContext(ctx); ok {
		err := tx.GetMulti(keys, dst)
		if err != nil {
			tx.Rollback()
			return err
		}

		return nil
	}

	return c.client.GetMulti(ctx, keys, dst)
}

func (c *wrappedClientImpl) Put(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.Key, error) {
	return c.client.Put(ctx, key, src)
}

func (c *wrappedClientImpl) PutWithTransaction(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.PendingKey, error) {
	if tx, ok := TransactionFromContext(ctx); ok {
		pendingKey, err := tx.Put(key, src)
		if err != nil {
			tx.Rollback()
			return pendingKey, err
		}

		return pendingKey, nil
	}

	return &datastore.PendingKey{}, fmt.Errorf("wrappedClientImpl: No transaction in context")
}

func (c *wrappedClientImpl) PutMulti(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.Key, err error) {
	return c.client.PutMulti(ctx, keys, src)
}

func (c *wrappedClientImpl) PutMultiWithTransaction(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.PendingKey, err error) {
	if tx, ok := TransactionFromContext(ctx); ok {
		pendingKeys, err := tx.PutMulti(keys, src)
		if err != nil {
			tx.Rollback()
			return pendingKeys, err
		}

		return pendingKeys, nil
	}

	return []*datastore.PendingKey{}, fmt.Errorf("wrappedClientImpl: No transaction in context")
}

func (c *wrappedClientImpl) RunInTransaction(ctx context.Context, f func(tx *datastore.Transaction) error) (*datastore.Commit, error) {
	return c.client.RunInTransaction(ctx, f)
}

func (c *wrappedClientImpl) Run(ctx context.Context, q *datastore.Query) *datastore.Iterator {
	return c.client.Run(ctx, q)
}

func (c *wrappedClientImpl) getUnderlyingClient() *datastore.Client {
	return c.client
}
