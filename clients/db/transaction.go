package db

import "cloud.google.com/go/datastore"

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

func NewTransaction(tx *datastore.Transaction) Transaction {
	return &transactionImpl{transaction: tx, IsPending: true}
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
