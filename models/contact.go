package models

import (
	"context"

	"github.com/hiconvo/api/db"
)

func GetContactsByUser(ctx context.Context, u *User) ([]*UserPartial, error) {
	// FIXME: Only get fields needed for UserPartial instead of getting everything
	// and then mapping to UserPartial.
	contacts := make([]*User, len(u.ContactKeys))
	if err := db.DefaultClient.GetMulti(ctx, u.ContactKeys, contacts); err != nil {
		return []*UserPartial{}, err
	}

	return MapUsersToUserPartials(contacts), nil
}
