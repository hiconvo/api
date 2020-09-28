package db

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/olivere/elastic/v7"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
	"github.com/hiconvo/api/model"
)

var _ model.NoteStore = (*NoteStore)(nil)

type NoteStore struct {
	DB db.Client
	S  search.Client
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
		if tag, ok := val.(string); ok {
			q = q.Filter("Tags =", tag)
		} else {
			return nil, errors.E(op, http.StatusBadRequest)
		}
	}

	if val, ok := m["filter"]; ok {
		if val == "note" {
			q = q.Filter("Variant =", "note")
		} else if val == "link" {
			q = q.Filter("Variant =", "link")
		}
	}

	if val, ok := m["search"]; ok {
		if len(m) > 1 {
			return nil, errors.E(op, errors.Str("search used with other params"),
				map[string]string{"message": "search cannot be combined with other parameters"},
				http.StatusBadRequest)
		}

		if query, ok := val.(string); ok {
			return s.handleSearch(ctx, u, query)
		}

		return nil, errors.E(op, http.StatusBadRequest)
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

	s.updateSearchIndex(ctx, n)

	return nil
}

func (s *NoteStore) Delete(ctx context.Context, n *model.Note) error {
	s.deleteSearchIndex(ctx, n)

	if err := s.DB.Delete(ctx, n.Key); err != nil {
		return err
	}

	return nil
}

func (s *NoteStore) handleSearch(ctx context.Context, u *model.User, q string) ([]*model.Note, error) {
	skip := 0
	take := 30

	notes := make([]*model.Note, 0)

	esQuery := elastic.NewBoolQuery().
		Must(elastic.NewMultiMatchQuery(q, "body", "name", "url")).
		Filter(elastic.NewTermQuery("userId.keyword", u.Key.Encode()))

	result, err := s.S.Search().
		Index("notes").
		Query(esQuery).
		From(skip).Size(take).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, hit := range result.Hits.Hits {
		note := new(model.Note)

		if err := json.Unmarshal(hit.Source, note); err != nil {
			return notes, err
		}

		notes = append(notes, note)
	}

	return notes, nil
}

func (s *NoteStore) updateSearchIndex(ctx context.Context, n *model.Note) {
	_, upsertErr := s.S.Update().
		Index("notes").
		Id(n.ID).
		DocAsUpsert(true).
		Doc(n).
		Do(ctx)
	if upsertErr != nil {
		log.Printf("Failed to index note in elasticsearch: %v", upsertErr)
	}
}

func (s *NoteStore) deleteSearchIndex(ctx context.Context, n *model.Note) {
	_, upsertErr := s.S.Delete().
		Index("notes").
		Id(n.ID).
		Do(ctx)
	if upsertErr != nil {
		log.Printf("Failed to delete note in elasticsearch: %v", upsertErr)
	}
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
