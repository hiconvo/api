package models

import (
	"context"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/db"
)

type DigestItem struct {
	ParentID *datastore.Key
	Name     string
	Messages []*Message
}

type Digestable interface {
	GetKey() *datastore.Key
	GetName() string
}

type DigestError struct{}

func (e *DigestError) Error() string {
	return "Nothing to digest"
}

func GenerateDigestList(ctx context.Context, digestables []Digestable, u *User) ([]DigestItem, error) {
	var digest []DigestItem
	for i := range digestables {
		item, err := GenerateDigestItem(ctx, digestables[i], u)
		if err != nil {
			switch err.(type) {
			case *DigestError:
				continue
			default:
				return digest, err
			}
		}

		digest = append(digest, item)
	}

	return digest, nil
}

func GenerateDigestItem(ctx context.Context, d Digestable, u *User) (DigestItem, error) {
	messages, err := GetMessagesByKey(ctx, d.GetKey())
	if err != nil {
		return DigestItem{}, err
	}

	var unread []*Message
	for j := range messages {
		if !IsRead(messages[j], u.Key) {
			unread = append(unread, messages[j])
		}
	}

	if len(unread) > 0 {
		return DigestItem{
			ParentID: d.GetKey(),
			Name:     d.GetName(),
			Messages: unread,
		}, nil
	}

	return DigestItem{}, &DigestError{}
}

func MarkDigestedMessagesAsRead(ctx context.Context, digestList []DigestItem, user *User) error {
	var messages []*Message
	var keys []*datastore.Key
	for i := range digestList {
		for j := range digestList[i].Messages {
			MarkAsRead(digestList[i].Messages[j], user.Key)
			messages = append(messages, digestList[i].Messages[j])
			keys = append(keys, digestList[i].Messages[j].Key)
		}
	}

	_, err := db.Client.PutMulti(ctx, keys, messages)
	if err != nil {
		return err
	}

	return nil
}
