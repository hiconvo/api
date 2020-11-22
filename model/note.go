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
	Variant   string         `json:"variant"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type GetNotesOption func(m map[string]interface{})

type NoteStore interface {
	GetNoteByID(ctx context.Context, id string) (*Note, error)
	GetNotesByUser(ctx context.Context, u *User, p *Pagination, o ...GetNotesOption) ([]*Note, error)
	Commit(ctx context.Context, n *Note) error
	Delete(ctx context.Context, n *Note) error
}

func NewNote(u *User, name, url, favicon, body string) (*Note, error) {
	op := errors.Op("model.NewNote")

	errMap := map[string]string{}
	var err error

	if len(url) == 0 && len(body) == 0 {
		errMap["body"] = "body cannot be empty without a url"
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

	if len(name) == 0 && len(body) > 0 {
		name = getNameFromBlurb(body)
	} else {
		name = getNameFromBlurb(name)
	}

	var variant string
	if len(url) > 0 {
		variant = "link"
	} else {
		variant = "note"
	}

	return &Note{
		Key:       datastore.IncompleteKey("Note", nil),
		OwnerKey:  u.Key,
		UserID:    u.Key.Encode(),
		Name:      name,
		URL:       url,
		Favicon:   favicon,
		Body:      body,
		Variant:   variant,
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

func (n *Note) AddTag(tag string) {
	for i := range n.Tags {
		if n.Tags[i] == tag {
			return
		}
	}

	n.Tags = append(n.Tags, tag)
}

func (n *Note) RemoveTag(tag string) {
	for i := range n.Tags {
		if n.Tags[i] == tag {
			n.Tags[i] = n.Tags[len(n.Tags)-1]
			n.Tags = n.Tags[:len(n.Tags)-1]
		}
	}
}

func (n *Note) UpdateNameFromBlurb(body string) {
	n.Name = getNameFromBlurb(body)
}

func getNameFromBlurb(body string) string {
	var name string

	trimmed := strings.TrimLeft(body, "#")
	trimmed = strings.TrimSpace(trimmed)
	split := strings.SplitAfterN(trimmed, "\n", 2)

	if len(split) > 0 {
		if len(split[0]) > 128 {
			name = split[0][:128]
		} else {
			name = split[0]
		}
	} else {
		if len(body) > 128 {
			name = body[:128]
		} else {
			name = body
		}
	}

	name = strings.TrimSpace(name)

	if len(name) == 0 {
		return "Everything is what it is and not another thing"
	}

	return name
}
