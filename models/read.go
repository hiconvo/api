package models

import (
	"time"

	"cloud.google.com/go/datastore"
)

type Read struct {
	UserKey   *datastore.Key
	Timestamp time.Time
}

type Readable interface {
	GetReads() []*Read
	SetReads([]*Read)
}

func NewRead(userKey *datastore.Key) *Read {
	return &Read{
		UserKey:   userKey,
		Timestamp: time.Now(),
	}
}

func MarkAsRead(r Readable, userKey *datastore.Key) {
	reads := r.GetReads()

	// If this has already been read, skip it
	if IsRead(r, userKey) {
		return
	}

	reads = append(reads, NewRead(userKey))

	r.SetReads(reads)
}

func ClearReads(r Readable) {
	r.SetReads([]*Read{})
}

func IsRead(r Readable, userKey *datastore.Key) bool {
	reads := r.GetReads()

	for i := range reads {
		if reads[i].UserKey.Equal(userKey) {
			return true
		}
	}

	return false
}

func MapReadsToUserPartials(r Readable, users []*User) []*UserPartial {
	reads := r.GetReads()
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
