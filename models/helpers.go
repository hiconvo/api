package models

import (
	"context"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
)

func mergeContacts(a, b []*datastore.Key) []*datastore.Key {
	var all []*datastore.Key
	all = append(all, a...)
	all = append(all, b...)

	var merged []*datastore.Key
	seen := make(map[string]struct{})

	for i := range all {
		keyString := all[i].String()

		if _, isSeen := seen[keyString]; isSeen {
			continue
		}

		seen[keyString] = struct{}{}
		merged = append(merged, all[i])
	}

	return merged
}

func reassignMessageUsers(ctx context.Context, old, new *User) error {
	oldUserMessages, err := GetUnhydratedMessagesByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of messages and save keys to oldUserMessageKeys slice
	oldUserMessageKeys := make([]*datastore.Key, len(oldUserMessages))
	for i := range oldUserMessages {
		oldUserMessages[i].UserKey = new.Key
		oldUserMessageKeys[i] = oldUserMessages[i].Key
	}

	// Save the messages
	_, err = db.Client.PutMulti(ctx, oldUserMessageKeys, oldUserMessages)
	if err != nil {
		return err
	}

	return nil
}

func reassignThreadUsers(ctx context.Context, old, new *User) error {
	oldUserThreads, err := GetUnhydratedThreadsByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of threads and save keys to oldUserThreadKeys slice
	oldUserThreadKeys := make([]*datastore.Key, len(oldUserThreads))
	for i := range oldUserThreads {
		oldUserThreads[i].RemoveUser(old)
		oldUserThreads[i].AddUser(new)

		if oldUserThreads[i].OwnerIs(old) {
			oldUserThreads[i].OwnerKey = new.Key
		}

		oldUserThreadKeys[i] = oldUserThreads[i].Key
	}

	// Save the threads
	_, err = db.Client.PutMulti(ctx, oldUserThreadKeys, oldUserThreads)
	if err != nil {
		return err
	}

	return nil
}

func reassignEventUsers(ctx context.Context, old, new *User) error {
	oldUserEvents, err := GetUnhydratedEventsByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of events and save keys to oldUserEvetKeys slice
	oldUserEventKeys := make([]*datastore.Key, len(oldUserEvents))
	for i := range oldUserEvents {
		oldUserEvents[i].RemoveUser(old)
		oldUserEvents[i].AddUser(new)

		if oldUserEvents[i].OwnerIs(old) {
			oldUserEvents[i].OwnerKey = new.Key
		}

		oldUserEventKeys[i] = oldUserEvents[i].Key
	}

	// Save the events
	_, err = db.Client.PutMulti(ctx, oldUserEventKeys, oldUserEvents)
	if err != nil {
		return err
	}

	return nil
}
