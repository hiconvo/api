package testutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/icrowley/fake"
	"go.mongodb.org/mongo-driver/mongo"

	dbc "github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/magic"
	sender "github.com/hiconvo/api/clients/mail"
	mgc "github.com/hiconvo/api/clients/mongo"
	"github.com/hiconvo/api/clients/notification"
	"github.com/hiconvo/api/clients/oauth"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/clients/places"
	"github.com/hiconvo/api/clients/queue"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/clients/storage"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/handler"
	"github.com/hiconvo/api/mail"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/template"
	"github.com/hiconvo/api/welcome"
)

type Mock struct {
	UserStore    model.UserStore
	ThreadStore  model.ThreadStore
	EventStore   model.EventStore
	MessageStore model.MessageStore
	NoteStore    model.NoteStore
	Welcome      model.Welcomer
	Mail         *mail.Client
	Magic        magic.Client
	OAuth        oauth.Client
	Storage      *storage.Client
	OG           opengraph.Client
	Places       places.Client
	Queue        queue.Client
}

func Handler(dbClient dbc.Client, searchClient search.Client) (http.Handler, *Mock) {
	mailClient := mail.New(sender.NewLogger(), template.NewClient())
	magicClient := magic.NewClient("")
	storageClient := storage.NewClient("", "")
	userStore := &db.UserStore{DB: dbClient, Notif: notification.NewLogger(), S: searchClient, Queue: queue.NewLogger()}
	threadStore := &db.ThreadStore{DB: dbClient, Storage: storageClient}
	eventStore := &db.EventStore{DB: dbClient}
	messageStore := &db.MessageStore{DB: dbClient, Storage: storageClient}
	noteStore := &db.NoteStore{DB: dbClient, S: searchClient}
	welcomer := welcome.New(context.Background(), userStore, "support")

	h := handler.New(&handler.Config{
		Transacter:    dbClient,
		UserStore:     userStore,
		ThreadStore:   threadStore,
		EventStore:    eventStore,
		MessageStore:  messageStore,
		NoteStore:     noteStore,
		Welcome:       welcomer,
		TxnMiddleware: dbc.WithTransaction(dbClient),
		Mail:          mailClient,
		Magic:         magicClient,
		Storage:       storageClient,
		OAuth:         oauth.NewClient(""),
		Notif:         notification.NewLogger(),
		OG:            opengraph.NewClient(),
		Places:        places.NewLogger(),
		Queue:         queue.NewLogger(),
	})

	m := &Mock{
		UserStore:    userStore,
		ThreadStore:  threadStore,
		EventStore:   eventStore,
		MessageStore: messageStore,
		NoteStore:    noteStore,
		Welcome:      welcomer,
		Mail:         mailClient,
		Magic:        magicClient,
		Storage:      storageClient,
		OAuth:        oauth.NewClient(""),
		OG:           opengraph.NewClient(),
		Places:       places.NewLogger(),
		Queue:        queue.NewLogger(),
	}

	return h, m
}

func (m *Mock) NewUser(ctx context.Context, t *testing.T) (*model.User, string) {
	t.Helper()

	email := fake.EmailAddress()
	pw := fake.SimplePassword()

	u, err := model.NewUserWithPassword(
		email,
		fake.FirstName(),
		fake.LastName(),
		pw)
	if err != nil {
		t.Fatal(err)
	}

	u.Verified = true

	err = m.UserStore.Commit(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	return u, pw
}

func (m *Mock) NewIncompleteUser(ctx context.Context, t *testing.T) *model.User {
	t.Helper()

	u, err := model.NewIncompleteUser(fake.EmailAddress())
	if err != nil {
		t.Fatal(err)
	}

	err = m.UserStore.Commit(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	return u
}

func (m *Mock) NewThread(
	ctx context.Context,
	t *testing.T,
	owner *model.User,
	users []*model.User,
) *model.Thread {
	t.Helper()

	th, err := model.NewThread(ctx, m.ThreadStore, m.Storage, m.OG, &model.NewThreadInput{
		Owner:   owner,
		Users:   users,
		Subject: fake.Title(),
		Body:    fake.Paragraph(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = m.ThreadStore.Commit(ctx, th)
	if err != nil {
		t.Fatal(err)
	}

	return th
}

func (m *Mock) NewEvent(
	ctx context.Context,
	t *testing.T,
	owner *model.User,
	hosts []*model.User,
	users []*model.User,
) *model.Event {
	t.Helper()

	ev, err := model.NewEvent(
		fake.Title(),
		fake.Paragraph(),
		fake.CharactersN(32),
		fake.StreetAddress(),
		0.0, 0.0,
		time.Date(2030, 6, 5, 4, 3, 2, 1, time.Local),
		0,
		owner,
		hosts,
		users,
		false)
	if err != nil {
		t.Fatal(err)
	}

	err = m.EventStore.Commit(ctx, ev)
	if err != nil {
		t.Fatal(err)
	}

	return ev
}

func (m *Mock) NewThreadMessage(
	ctx context.Context,
	t *testing.T,
	owner *model.User,
	thread *model.Thread,
) *model.Message {
	t.Helper()

	mess, err := model.NewThreadMessage(
		ctx,
		m.Storage,
		m.OG,
		&model.NewMessageInput{
			User:   owner,
			Parent: thread.Key,
			Body:   fake.Paragraph(),
			Blob:   "",
		})
	if err != nil {
		t.Fatal(err)
	}

	err = m.MessageStore.Commit(ctx, mess)
	if err != nil {
		t.Fatal(err)
	}

	return mess
}

func (m *Mock) NewEventMessage(
	ctx context.Context,
	t *testing.T,
	owner *model.User,
	event *model.Event,
) *model.Message {
	t.Helper()

	mess, err := model.NewEventMessage(owner, event, fake.Paragraph(), "", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = m.MessageStore.Commit(ctx, mess)
	if err != nil {
		t.Fatal(err)
	}

	return mess
}

func (m *Mock) NewNote(ctx context.Context, t *testing.T, u *model.User) *model.Note {
	t.Helper()

	n, err := model.NewNote(u, fake.Title(), "", "", fake.Paragraph())
	if err != nil {
		t.Fatal(err)
	}

	err = m.NoteStore.Commit(ctx, n)
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func NewNotifClient(t *testing.T) notification.Client {
	t.Helper()
	return notification.NewLogger()
}

func NewUserStore(ctx context.Context, t *testing.T, dbClient dbc.Client, searchClient search.Client) model.UserStore {
	t.Helper()
	return &db.UserStore{DB: dbClient, Notif: notification.NewLogger(), S: searchClient}
}

func NewThreadStore(ctx context.Context, t *testing.T, dbClient dbc.Client) model.ThreadStore {
	t.Helper()
	return &db.ThreadStore{DB: dbClient}
}

func NewMessageStore(ctx context.Context, t *testing.T, dbClient dbc.Client) model.MessageStore {
	t.Helper()
	return &db.MessageStore{DB: dbClient}
}

func NewEventStore(ctx context.Context, t *testing.T, dbClient dbc.Client) model.EventStore {
	t.Helper()
	return &db.EventStore{DB: dbClient}
}

func NewNoteStore(ctx context.Context, t *testing.T, dbClient dbc.Client, searchClient search.Client) model.NoteStore {
	t.Helper()
	return &db.NoteStore{DB: dbClient, S: searchClient}
}

func NewSearchClient() search.Client {
	esh := os.Getenv("ELASTICSEARCH_HOST")
	if esh == "" {
		esh = "elasticsearch"
	}

	return search.NewClient(esh)
}

func NewDBClient(ctx context.Context) dbc.Client {
	return dbc.NewClient(ctx, "local-convo-api")
}

func NewMongoClient(ctx context.Context) (*mongo.Client, func()) {
	conn := os.Getenv("MONGO_CONNECTION")
	if conn == "" {
		conn = "mongo"
	}
	c, closer := mgc.NewClient(ctx, conn)
	return c, closer
}

func ClearDB(ctx context.Context, client dbc.Client) {
	for _, tp := range []string{"User", "Thread", "Event", "Message", "Note"} {
		q := datastore.NewQuery(tp).KeysOnly()

		keys, err := client.GetAll(ctx, q, nil)
		if err != nil {
			panic(err)
		}

		err = client.DeleteMulti(ctx, keys)
		if err != nil {
			panic(err)
		}
	}
}

func GetMagicLinkParts(link string) (string, string, string) {
	split := strings.Split(link, "/")
	kenc := split[len(split)-3]
	b64ts := split[len(split)-2]
	sig := split[len(split)-1]
	return kenc, b64ts, sig
}

func GetAuthHeader(token string) map[string]string {
	return map[string]string{"Authorization": fmt.Sprintf("Bearer %s", token)}
}
