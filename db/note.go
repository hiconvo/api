package db

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/model"
)

var _ model.NoteStore = (*NoteStore)(nil)

type NoteStore struct {
	DB  *mongo.Client
	Col *mongo.Collection
}

func NewNoteStore(client *mongo.Client) *NoteStore {
	return &NoteStore{DB: client, Col: client.Database("test").Collection("notes")}
}

func (s *NoteStore) GetNoteByID(ctx context.Context, id string) (*model.Note, error) {
	op := errors.Opf("NoteStore.GetNoteByID(id=%s)", id)

	docID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.E(op, err, http.StatusNotFound)
	}

	n := new(model.Note)

	err = s.Col.FindOne(ctx, bson.M{"_id": docID}).Decode(n)
	if err != nil {
		return nil, errors.E(op, err, http.StatusNotFound)
	}

	n.ID = id
	n.Key = docID

	return n, nil
}

func (s *NoteStore) GetNotesByUser(
	ctx context.Context,
	u *model.User,
	p *model.Pagination,
	opts ...model.GetNotesOption,
) ([]*model.Note, error) {
	op := errors.Opf("NoteStore.GetNotesByUser(u=%s)", u.Email)

	m := map[string]interface{}{"ownerid": u.ID}

	for _, f := range opts {
		f(m)
	}

	cur, err := s.Col.Find(ctx, bson.M(m), options.Find().
		SetSort(bson.M{"createdat": -1}).
		SetLimit(int64(p.Limit())).
		SetSkip(int64(p.Offset())))
	if err != nil {
		return nil, errors.E(op, err)
	}

	notes := make([]*model.Note, cur.RemainingBatchLength())

	err = cur.All(ctx, &notes)
	if err != nil {
		return nil, errors.E(op, err)
	}

	for i := range notes {
		notes[i].ID = notes[i].Key.Hex()
	}

	return notes, nil
}

func (s *NoteStore) Commit(ctx context.Context, n *model.Note) error {
	op := errors.Op("NoteStore.Commit")

	if n.Key.IsZero() {
		res, err := s.Col.InsertOne(ctx, n)
		if err != nil {
			return errors.E(op, err)
		}

		n.Key = res.InsertedID.(primitive.ObjectID)
		n.ID = n.Key.Hex()
	} else {
		_, err := s.Col.ReplaceOne(ctx, bson.M{"_id": n.Key}, n)
		if err != nil {
			return errors.E(op, err)
		}
	}

	return nil
}

func (s *NoteStore) Delete(ctx context.Context, n *model.Note) error {
	op := errors.Opf("NoteStore.Commit(id=%s)", n.ID)

	_, err := s.Col.DeleteOne(ctx, bson.M{"_id": n.Key})
	if err != nil {
		return errors.E(op, err)
	}

	return nil
}

func GetNotesFilter(val string) model.GetNotesOption {
	return func(m map[string]interface{}) {
		if len(val) > 0 {
			m["filter"] = val
		}
	}
}

func GetNotesSearch(val string) model.GetNotesOption {
	return func(m map[string]interface{}) {
		if len(val) > 0 {
			m["$text"] = bson.M{"$search": val}
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
