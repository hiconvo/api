package models

import (
	"context"
	"fmt"
	"os"

	"github.com/hiconvo/api/utils/secrets"
)

var supportUser *User
var welcomeMessage string = `Hello 👋,

Welcome to Convo! Convo has two main features, **events** and **messaging**.

Convo events make it easy to plan events with real people. Invite your guests by name or email and they can RSVP in one click without having to create accounts of their own.

Convo also allows you to message with people directly via *Convos*. A Convo is a thin abstraction layer over email that makes it easy to connect with people by their real names without revealing any personal contact info.

Read more about Convo and why I built it on [the blog](https://blog.hiconvo.com/hello-world).

If you find any bugs 🐛or have any suggestions or feedback, please respond to this Convo directly and I'll get back to you.

Thanks,

Alex`

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

		fmt.Fprintf(os.Stderr, "Created new support user\n")
	}

	supportUser = &u
}
