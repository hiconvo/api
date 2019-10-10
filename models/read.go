package models

import (
	"time"

	"cloud.google.com/go/datastore"
)

type Read struct {
	Key       *datastore.Key `datastore:"__key__"`
	UserKey   *datastore.Key
	Timestamp time.Time
}

type Readable interface {
	Reads() []*Read
	SetReads([]*Read)
}

func NewRead(userKey *datastore.Key) *Read {
	return &Read{
		Key:       datastore.IncompleteKey("Read", nil),
		UserKey:   userKey,
		Timestamp: time.Now(),
	}
}

func MarkAsRead(r Readable, userKey *datastore.Key) {
	reads := r.Reads()

	// If this has already been read, skip it
	for i := range reads {
		if reads[i].UserKey.Equal(userKey) {
			return
		}
	}

	reads = append(reads, NewRead(userKey))

	r.SetReads(reads)
}

func ClearReads(r Readable) {
	r.SetReads([]*Read{})
}

func MapReadsToUserPartials(r Readable, users []*User) []*UserPartial {
	reads := r.Reads()
	userPartials := make([]*UserPartial, len(reads))
	for i := range reads {
		for j := range users {
			if users[j].Key.Equal(reads[i].UserKey) {
				userPartials[i] = MapUserToUserPartial(users[j])
				break
			}
		}
	}

	return userPartials
}
