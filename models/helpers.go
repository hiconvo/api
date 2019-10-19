package models

import (
	"cloud.google.com/go/datastore"
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
