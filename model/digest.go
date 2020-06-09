package model

import "cloud.google.com/go/datastore"

type DigestItem struct {
	ParentID *datastore.Key
	Name     string
	Messages []*Message
}

type Digestable interface {
	GetKey() *datastore.Key
	GetName() string
}
