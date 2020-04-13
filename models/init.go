package models

import (
	"context"

	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/utils/secrets"
)

var (
	_supportUser    *User
	_welcomeMessage string = readStringFromFile("welcome.md")
)

func init() {
	supportPassword := secrets.Get("SUPPORT_PASSWORD", "support")
	ctx := context.Background()

	u, found, err := GetUserByEmail(ctx, "support@convo.events")
	if err != nil {
		panic(err)
	}

	if !found {
		u, err = NewUserWithPassword("support@convo.events", "Convo Support", "", supportPassword)
		if err != nil {
			panic(err)
		}

		err = u.Commit(ctx)
		if err != nil {
			panic(err)
		}

		log.Print("models.init: Created new support user")
	}

	_supportUser = &u
}
