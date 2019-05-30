package models

import "errors"

type Contact struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
}

func MapUserToContact(u *User) *Contact {
	return &Contact{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		FullName:  u.FullName,
	}
}

func MapUsersToContacs(users []*User) []*Contact {
	contacts := make([]*Contact, len(users))
	for i, u := range users {
		contacts[i] = MapUserToContact(u)
	}
	return contacts
}

func MapContactToUser(c *Contact, users []*User) (*User, error) {
	for _, u := range users {
		if u.ID == c.ID {
			return u, nil
		}
	}

	return &User{}, errors.New("Matching user not in slice")
}
