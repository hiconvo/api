package model

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hiconvo/api/errors"
)

type TagPair struct {
	Name  string
	Count int
}

type TagList []TagPair

func (t TagList) MarshalJSON() ([]byte, error) {
	tags := make([]string, len(t))

	for i, pair := range t {
		tags[i] = pair.Name
	}

	return json.Marshal(tags)
}

func (t *TagList) Add(tag string) {
	list := *t

	for i := range list {
		if list[i].Name == tag {
			list[i].Count++

			*t = list

			return
		}
	}

	list = append(list, TagPair{Name: tag, Count: 1})

	*t = list
}

func (t *TagList) Remove(tag string) {
	list := *t

	for i := range list {
		if list[i].Name == tag {
			list[i].Count--

			if list[i].Count > 0 {
				*t = list

				return
			}

			list[i] = list[len(list)-1]
			list = list[:len(list)-1]

			*t = list

			return
		}
	}
}

func TabulateNoteTags(u *User, n *Note, dirtyTags []string) (userChanged bool, err error) {
	op := errors.Op("model.TabulateNoteTags")

	newTags := make([]string, 0)
	for i := range dirtyTags {
		if i > 3 {
			return false, errors.E(op, errors.Str("too many tags"),
				map[string]string{"message": "only four tags are supported at this time"},
				http.StatusBadRequest)
		}

		if len(dirtyTags[i]) > 12 {
			return false, errors.E(op, errors.Str("tag too long"),
				map[string]string{"message": "tags can only be 12 characters long"},
				http.StatusBadRequest)
		}

		newTags = append(newTags, strings.ToLower(dirtyTags[i]))
	}

	added := make([]string, 0)
	for i := range newTags {
		found := false
		for j := range n.Tags {
			if newTags[i] == n.Tags[j] {
				found = true
				break
			}
		}

		if !found {
			added = append(added, newTags[i])
		}
	}

	removed := make([]string, 0)
	for i := range n.Tags {
		found := false
		for j := range newTags {
			if n.Tags[i] == newTags[j] {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, n.Tags[i])
		}
	}

	for i := range added {
		u.Tags.Add(added[i])
	}

	for i := range removed {
		u.Tags.Remove(removed[i])
	}

	if len(u.Tags) > 60 {
		return false, errors.E(op, errors.Str("too many unique tags"),
			map[string]string{"message": "you have too many unique tags"},
			http.StatusBadRequest)
	}

	n.Tags = newTags

	if len(added) > 0 || len(removed) > 0 {
		return true, nil
	}

	return false, nil
}
