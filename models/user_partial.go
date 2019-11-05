package models

import (
	"errors"
	"strings"
)

type UserPartial struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
	Avatar    string `json:"avatar"`
}

func MapUserToUserPartial(u *User) *UserPartial {
	// If the user does not have any name info, show the part of their email
	// before the "@"
	var fullName string
	if u.FirstName == "" && u.LastName == "" && u.FullName == "" {
		fullName = strings.Split(u.Email, "@")[0]
	} else {
		fullName = u.FullName
	}

	return &UserPartial{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  fullName,
		Avatar:    u.Avatar,
	}
}

func MapUsersToUserPartials(users []*User) []*UserPartial {
	contacts := make([]*UserPartial, len(users))
	for i, u := range users {
		contacts[i] = MapUserToUserPartial(u)
	}
	return contacts
}

func MapUserPartialToUser(c *UserPartial, users []*User) (*User, error) {
	for _, u := range users {
		if u.ID == c.ID {
			return u, nil
		}
	}

	return &User{}, errors.New("Matching user not in slice")
}
