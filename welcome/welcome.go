package welcome

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/model"
)

var _ model.Welcomer = (*Welcomer)(nil)

type Welcomer struct {
	supportUser    *model.User
	welcomeMessage string
	nullOG         opengraph.Client
}

func New(ctx context.Context, us model.UserStore, supportPassword string) *Welcomer {
	op := errors.Op("welcome.New")

	spuser, found, err := us.GetUserByEmail(ctx, "support@convo.events")
	if err != nil {
		panic(errors.E(op, err))
	}

	if !found {
		spuser, err = model.NewUserWithPassword(
			"support@convo.events", "Convo Support", "", supportPassword)
		if err != nil {
			panic(errors.E(op, err))
		}

		err = us.Commit(ctx, spuser)
		if err != nil {
			panic(errors.E(op, err))
		}

		log.Print("welcome.New: Created new support user")
	}

	return &Welcomer{
		supportUser:    spuser,
		welcomeMessage: readStringFromFile("welcome.md"),
		nullOG:         opengraph.NewNullClient(),
	}
}

func (w *Welcomer) Welcome(
	ctx context.Context,
	ts model.ThreadStore,
	sclient *storage.Client,
	u *model.User,
) error {
	var op errors.Op = "user.Welcome"

	thread, err := model.NewThread(
		ctx,
		ts,
		sclient,
		w.nullOG,
		&model.NewThreadInput{
			Owner:   w.supportUser,
			Users:   []*model.User{u},
			Subject: "Welcome",
			Body:    w.welcomeMessage,
		})
	if err != nil {
		return errors.E(op, err)
	}

	// Don't spam users with this welcome message in their digests
	model.MarkAsRead(thread, u.Key)

	if err := ts.Commit(ctx, thread); err != nil {
		return errors.E(op, err)
	}

	log.Printf("welcome.Welcome: created welcome thread for %q", u.Email)

	return nil
}

func readStringFromFile(file string) string {
	op := errors.Opf("welcome.readStringFromFile(file=%s)", file)

	wd, err := os.Getwd()
	if err != nil {
		// This function should only be run at startup time, so we
		// panic if it fails.
		panic(errors.E(op, err))
	}

	var basePath string
	if strings.HasSuffix(wd, "welcome") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// content.
		basePath = "../welcome/content"
	} else {
		basePath = "./welcome/content"
	}

	b, err := ioutil.ReadFile(path.Join(basePath, file))
	if err != nil {
		panic(err)
	}

	return string(b)
}
