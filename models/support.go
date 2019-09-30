package models

import (
	"context"

	"github.com/hiconvo/api/utils/secrets"
)

var supportUser *User
var welcomeMessage string = "Welcome to Convo.\n\nSoon this welcome convo will contain helpful info about what convo is and how to use it. But right now, it does not."

func init() {
	supportPw := secrets.Get("SUPPORT_PASSWORD", "support")
	ctx := context.Background()

	u, found, err := GetUserByEmail(ctx, "support@hiconvo.com")
	if err != nil {
		panic(err)
	}

	if !found {
		u, err = NewUserWithPassword("support@hiconvo.com", "Convo Support", "", supportPw)
		if err != nil {
			panic(err)
		}
		err = u.Commit(ctx)
		if err != nil {
			panic(err)
		}
	}

	supportUser = &u
}
