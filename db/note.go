package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

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
		return nil, errors.E(op, err)
	}

	n := new(model.Note)

	err = s.Col.FindOne(ctx, bson.M{"_id": docID}).Decode(n)
	if err != nil {
		return nil, errors.E(op, err)
	}

	n.ID = id
	n.Key = docID

	return n, nil
}

func (s *NoteStore) GetNotesByUser(ctx context.Context, u *model.User, p *model.Pagination) ([]*model.Note, error) {
	return nil, nil
}

func (s *NoteStore) Commit(ctx context.Context, n *model.Note) error {
	op := errors.Op("NoteStore.Commit")

	res, err := s.Col.InsertOne(ctx, n)
	if err != nil {
		return errors.E(op, err)
	}

	n.Key = res.InsertedID.(primitive.ObjectID)
	n.ID = n.Key.Hex()

	return nil
}

func (s *NoteStore) Delete(ctx context.Context, n *model.Note) error {
	return nil
}
