package model

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/valid"
)

type Note struct {
	Key       primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	ID        string             `json:"id"`
	OwnerID   string             `json:"-"`
	Body      string             `json:"body"`
	Tags      []string           `json:"tags"`
	URL       string             `json:"url"`
	Favicon   string             `json:"favicon"`
	Name      string             `json:"name"`
	CreatedAt time.Time          `json:"createdAt"`
}

type NoteStore interface {
	GetNoteByID(ctx context.Context, id string) (*Note, error)
	GetNotesByUser(ctx context.Context, u *User, p *Pagination) ([]*Note, error)
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
		split := strings.SplitAfterN(body, "\n", 1)
		if len(split) > 0 {
			name = split[0][:255]
		} else {
			name = body[:255]
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
		return nil, errors.E(op, errMap, http.StatusBadRequest)
	}

	return &Note{
		OwnerID: u.ID,
		Name:    name,
		URL:     url,
		Favicon: favicon,
		Body:    body,
		Tags:    tags,
	}, nil
}
