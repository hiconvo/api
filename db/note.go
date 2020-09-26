package db

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
)

var _ model.NoteStore = (*NoteStore)(nil)

type NoteStore struct {
	DB db.Client
}

func (s *NoteStore) GetNoteByID(ctx context.Context, id string) (*model.Note, error) {
	op := errors.Opf("NoteStore.GetNoteByID(id=%s)", id)
	note := new(model.Note)

	key, err := datastore.DecodeKey(id)
	if err != nil {
		return nil, errors.E(op, err)
	}

	err = s.DB.Get(ctx, key, note)
	if err != nil {
		return nil, errors.E(op, err)
	}

	return note, nil
}

func (s *NoteStore) GetNotesByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
	opts ...model.GetNotesOption,
) ([]*model.Note, error) {
	op := errors.Opf("NoteStore.GetNotesByUser(u=%s)", u.Email)

	notes := make([]*model.Note, 0)

	q := datastore.NewQuery("Note").
		Filter("OwnerKey =", u.Key).
		Order("-CreatedAt").
		Offset(p.Offset()).
		Limit(p.Limit())

	m := map[string]interface{}{}

	for _, f := range opts {
		f(m)
	}

	if _, ok := m["pins"]; ok {
		q = q.Filter("Pin =", true)
	}

	if val, ok := m["tags"]; ok {
		q = q.Filter("Tags =", val)
	}

	if val, ok := m["filter"]; ok {
		if val == "note" {
			q = q.Filter("URL =", "")
		} else if val == "link" {
			q = q.Filter("URL !=", "")
		}
	}

	if _, ok := m["search"]; ok {
		return nil, errors.E(op, errors.Str("Not implemented"))
	}

	_, err := s.DB.GetAll(ctx, q, &notes)
	if err != nil {
		return notes, errors.E(op, err)
	}

	return notes, nil
}

func (s *NoteStore) Commit(ctx context.Context, n *model.Note) error {
	op := errors.Op("NoteStore.Commit")

	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	key, err := s.DB.Put(ctx, n.Key, n)
	if err != nil {
		return errors.E(op, err)
	}

	n.ID = key.Encode()
	n.Key = key

	return nil
}

func (s *NoteStore) Delete(ctx context.Context, n *model.Note) error {
	if err := s.DB.Delete(ctx, n.Key); err != nil {
		return err
	}

	return nil
}

func GetNotesFilter(val string) model.GetNotesOption {
	return func(m map[string]interface{}) {
		if len(val) > 0 {
			m["filter"] = strings.ToLower(val)
		}
	}
}

func GetNotesSearch(val string) model.GetNotesOption {
	return func(m map[string]interface{}) {
		if len(val) > 0 {
			m["search"] = strings.ToLower(val)
		}
	}
}

func GetNotesTags(val string) model.GetNotesOption {
	return func(m map[string]interface{}) {
		if len(val) > 0 {
			m["tags"] = strings.ToLower(val)
		}
	}
}

func GetNotesPins() model.GetNotesOption {
	return func(m map[string]interface{}) {
		m["pins"] = true
	}
}
