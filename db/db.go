package db

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/datastore"
)

var DefaultClient Client

func init() {
	DefaultClient = NewClient(context.Background(), os.Getenv("PROJECT_ID"))
}

type Client interface {
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
	NewTransaction(ctx context.Context) (Transaction, error)
}

func NewClient(ctx context.Context, projectID string) Client {
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		panic(err)
	}

	return &clientImpl{client: client}
}

// clientImpl is a shim around datastore.Client that detects
// if a transaction is available on the current context and uses it
// if there is. Otherwise, it calls the corresponding
// datastore.Client method.
type clientImpl struct {
	client *datastore.Client
}

func (c *clientImpl) Count(ctx context.Context, q *datastore.Query) (int, error) {
	return c.client.Count(ctx, q)
}

func (c *clientImpl) Delete(ctx context.Context, key *datastore.Key) error {
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

func (c *clientImpl) DeleteMulti(ctx context.Context, keys []*datastore.Key) error {
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

func (c *clientImpl) Get(ctx context.Context, key *datastore.Key, dst interface{}) error {
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

func (c *clientImpl) GetAll(ctx context.Context, q *datastore.Query, dst interface{}) (keys []*datastore.Key, err error) {
	return c.client.GetAll(ctx, q, dst)
}

func (c *clientImpl) GetMulti(ctx context.Context, keys []*datastore.Key, dst interface{}) (err error) {
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

func (c *clientImpl) Put(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.Key, error) {
	return c.client.Put(ctx, key, src)
}

func (c *clientImpl) PutWithTransaction(ctx context.Context, key *datastore.Key, src interface{}) (*datastore.PendingKey, error) {
	if tx, ok := TransactionFromContext(ctx); ok {
		pendingKey, err := tx.Put(key, src)
		if err != nil {
			tx.Rollback()
			return pendingKey, err
		}

		return pendingKey, nil
	}

	return &datastore.PendingKey{}, fmt.Errorf("clientImpl: No transaction in context")
}

func (c *clientImpl) PutMulti(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.Key, err error) {
	return c.client.PutMulti(ctx, keys, src)
}

func (c *clientImpl) PutMultiWithTransaction(ctx context.Context, keys []*datastore.Key, src interface{}) (ret []*datastore.PendingKey, err error) {
	if tx, ok := TransactionFromContext(ctx); ok {
		pendingKeys, err := tx.PutMulti(keys, src)
		if err != nil {
			tx.Rollback()
			return pendingKeys, err
		}

		return pendingKeys, nil
	}

	return []*datastore.PendingKey{}, fmt.Errorf("clientImpl: No transaction in context")
}

func (c *clientImpl) RunInTransaction(ctx context.Context, f func(tx *datastore.Transaction) error) (*datastore.Commit, error) {
	return c.client.RunInTransaction(ctx, f)
}

func (c *clientImpl) Run(ctx context.Context, q *datastore.Query) *datastore.Iterator {
	return c.client.Run(ctx, q)
}

func (c *clientImpl) NewTransaction(ctx context.Context) (Transaction, error) {
	tx, err := c.client.NewTransaction(ctx)

	return &transactionImpl{transaction: tx, IsPending: true}, err
}

// Transaction is a wrapper around datastore.Transaction. It adds a Pending() method that
// allows it to be detected whether a transaction has been completed so that they are not
// accidentally left hanging.
type Transaction interface {
	Commit() (c *datastore.Commit, err error)
	Delete(key *datastore.Key) error
	DeleteMulti(keys []*datastore.Key) (err error)
	Get(key *datastore.Key, dst interface{}) (err error)
	GetMulti(keys []*datastore.Key, dst interface{}) (err error)
	Mutate(muts ...*datastore.Mutation) ([]*datastore.PendingKey, error)
	Put(key *datastore.Key, src interface{}) (*datastore.PendingKey, error)
	PutMulti(keys []*datastore.Key, src interface{}) (ret []*datastore.PendingKey, err error)
	Rollback() (err error)
	Pending() bool
}

type transactionImpl struct {
	transaction *datastore.Transaction

	IsPending bool
}

func (t *transactionImpl) Commit() (c *datastore.Commit, err error) {
	cm, err := t.transaction.Commit()
	if err != nil {
		return nil, err
	}

	t.IsPending = false

	return cm, err
}

func (t *transactionImpl) Delete(key *datastore.Key) error {
	return t.transaction.Delete(key)
}

func (t *transactionImpl) DeleteMulti(keys []*datastore.Key) error {
	return t.transaction.DeleteMulti(keys)
}

func (t *transactionImpl) Get(key *datastore.Key, dst interface{}) error {
	return t.transaction.Get(key, dst)
}

func (t *transactionImpl) GetMulti(keys []*datastore.Key, dst interface{}) error {
	return t.transaction.GetMulti(keys, dst)
}

func (t *transactionImpl) Mutate(muts ...*datastore.Mutation) ([]*datastore.PendingKey, error) {
	return t.transaction.Mutate(muts...)
}

func (t *transactionImpl) Put(key *datastore.Key, src interface{}) (*datastore.PendingKey, error) {
	return t.transaction.Put(key, src)
}

func (t *transactionImpl) PutMulti(keys []*datastore.Key, src interface{}) ([]*datastore.PendingKey, error) {
	return t.transaction.PutMulti(keys, src)
}

func (t *transactionImpl) Rollback() error {
	err := t.transaction.Rollback()
	if err != nil {
		return err
	}

	t.IsPending = false

	return nil
}

func (t *transactionImpl) Pending() bool {
	return t.IsPending
}
