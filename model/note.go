package model

import (
	"context"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/valid"
)

type Note struct {
	Key       *datastore.Key `json:"-"        datastore:"__key__"`
	ID        string         `json:"id"       datastore:"-"`
	OwnerKey  *datastore.Key `json:"-"`
	UserID    string         `json:"userId"   datastore:"-"`
	Body      string         `json:"body"     datastore:",noindex"`
	Tags      []string       `json:"tags"`
	URL       string         `json:"url"`
	Favicon   string         `json:"favicon"  datastore:",noindex"`
	Name      string         `json:"name"`
	Pin       bool           `json:"pin"`
	CreatedAt time.Time      `json:"createdAt"`
}

type GetNotesOption func(m map[string]interface{})

type NoteStore interface {
	GetNoteByID(ctx context.Context, id string) (*Note, error)
	GetNotesByUser(ctx context.Context, u *User, p *Pagination, o ...GetNotesOption) ([]*Note, error)
	Commit(ctx context.Context, n *Note) error
	Delete(ctx context.Context, n *Note) error
}

func NewNote(u *User, name, url, favicon, body string, tags []string) (*Note, error) {
	op := errors.Op("model.NewNote")

	errMap := map[string]string{}
	var err error

	if len(url) == 0 && len(body) == 0 {
		errMap["body"] = "body cannot be empty without a url"
	}

	if len(name) == 0 && len(body) > 0 {
		split := strings.SplitAfterN(body, "\n", 2)
		if len(split) > 0 {
			if len(split[0]) > 255 {
				name = split[0][:255]
			} else {
				name = split[0]
			}
		} else {
			if len(body) > 255 {
				name = body[:255]
			} else {
				name = body
			}
		}
	}

	if len(url) > 0 {
		url, err = valid.URL(url)
		if err != nil {
			errMap["url"] = "invalid url"
		}
	}

	if len(favicon) > 0 {
		favicon, err = valid.URL(favicon)
		if err != nil {
			errMap["favicon"] = "invalid url"
		}
	}

	if len(errMap) > 0 {
		return nil, errors.E(op, errMap,
			errors.Str("failed validation"), http.StatusBadRequest)
	}

	return &Note{
		Key:       datastore.IncompleteKey("Note", nil),
		OwnerKey:  u.Key,
		UserID:    u.Key.Encode(),
		Name:      name,
		URL:       url,
		Favicon:   favicon,
		Body:      body,
		Tags:      tags,
		CreatedAt: time.Now(),
	}, nil
}

func (n *Note) LoadKey(k *datastore.Key) error {
	n.Key = k

	// Add URL safe key
	n.ID = k.Encode()

	return nil
}

func (n *Note) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(n)
}

func (n *Note) Load(ps []datastore.Property) error {
	op := errors.Op("note.Load")

	err := datastore.LoadStruct(n, ps)
	if err != nil {
		return errors.E(op, err)
	}

	for _, p := range ps {
		if p.Name == "OwnerKey" {
			k, ok := p.Value.(*datastore.Key)
			if !ok {
				return errors.E(op, errors.Errorf("could not load owner key into note='%v'", n.ID))
			}

			n.OwnerKey = k
			n.UserID = k.Encode()
		}
	}

	return nil
}
