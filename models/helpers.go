package models

import (
	"context"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
)

func swapKeys(keyList []*datastore.Key, oldKey, newKey *datastore.Key) []*datastore.Key {
	for i := range keyList {
		if keyList[i].Equal(oldKey) {
			keyList[i] = newKey
		}
	}

	// Remove duplicates
	var clean []*datastore.Key
	seen := map[string]struct{}{}
	for i := range keyList {
		keyString := keyList[i].String()
		if _, hasVal := seen[keyString]; !hasVal {
			seen[keyString] = struct{}{}
			clean = append(clean, keyList[i])
		}
	}

	return clean
}

func swapReadUserKeys(readList []*Read, oldKey, newKey *datastore.Key) []*Read {
	for i := range readList {
		if readList[i].UserKey.Equal(oldKey) {
			readList[i].UserKey = newKey
		}
	}

	return readList
}

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

func reassignContacts(ctx context.Context, oldUser, newUser *User) error {
	var users []*User
	q := datastore.NewQuery("User").Filter("ContactKeys =", oldUser.Key)
	keys, err := db.Client.GetAll(ctx, q, &users)
	if err != nil {
		return err
	}

	for i := range users {
		swapKeys(users[i].ContactKeys, oldUser.Key, newUser.Key)
	}

	_, err = db.Client.PutMulti(ctx, keys, users)
	if err != nil {
		return err
	}

	return nil
}

func reassignMessageUsers(ctx context.Context, old, newUser *User) error {
	userMessages, err := GetUnhydratedMessagesByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of messages and save keys to oldUserMessageKeys slice
	userMessageKeys := make([]*datastore.Key, len(userMessages))
	for i := range userMessages {
		userMessages[i].UserKey = newUser.Key
		swapReadUserKeys(userMessages[i].Reads, old.Key, newUser.Key)
		userMessageKeys[i] = userMessages[i].Key
	}

	// Save the messages
	_, err = db.Client.PutMulti(ctx, userMessageKeys, userMessages)
	if err != nil {
		return err
	}

	return nil
}

func reassignThreadUsers(ctx context.Context, old, newUser *User) error {
	userThreads, err := GetUnhydratedThreadsByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of threads and save keys to oldUserThreadKeys slice
	userThreadKeys := make([]*datastore.Key, len(userThreads))
	for i := range userThreads {
		swapKeys(userThreads[i].UserKeys, old.Key, newUser.Key)
		swapReadUserKeys(userThreads[i].Reads, old.Key, newUser.Key)

		if userThreads[i].OwnerKey.Equal(old.Key) {
			userThreads[i].OwnerKey = newUser.Key
		}

		userThreadKeys[i] = userThreads[i].Key
	}

	// Save the threads
	_, err = db.Client.PutMulti(ctx, userThreadKeys, userThreads)
	if err != nil {
		return err
	}

	return nil
}

func reassignEventUsers(ctx context.Context, old, newUser *User) error {
	userEvents, err := GetUnhydratedEventsByUser(ctx, old)
	if err != nil {
		return err
	}

	// Reassign ownership of events and save keys to userEvetKeys slice
	userEventKeys := make([]*datastore.Key, len(userEvents))
	for i := range userEvents {
		swapKeys(userEvents[i].UserKeys, old.Key, newUser.Key)
		swapKeys(userEvents[i].RSVPKeys, old.Key, newUser.Key)
		swapReadUserKeys(userEvents[i].Reads, old.Key, newUser.Key)

		if userEvents[i].OwnerKey.Equal(old.Key) {
			userEvents[i].OwnerKey = newUser.Key
		}

		userEventKeys[i] = userEvents[i].Key
	}

	// Save the events
	_, err = db.Client.PutMulti(ctx, userEventKeys, userEvents)
	if err != nil {
		return err
	}

	return nil
}
