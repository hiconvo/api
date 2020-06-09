package contact

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/model"
)

type Config struct {
	UserStore model.UserStore
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.WithUser(c.UserStore))
	r.HandleFunc("/contacts", c.GetContacts).Methods("GET")
	r.HandleFunc("/contacts/{userID}", c.AddContact).Methods("POST")
	r.HandleFunc("/contacts/{userID}", c.RemoveContact).Methods("DELETE")

	return r
}

func (c *Config) GetContacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	contacts, err := c.UserStore.GetContactsByUser(ctx, u)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w,
		map[string][]*model.UserPartial{"contacts": model.MapUsersToUserPartials(contacts)},
		http.StatusOK)
}

func (c *Config) AddContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	if !u.IsRegistered() {
		bjson.HandleError(w, errors.E(
			errors.Op("handlers.AddContact"),
			errors.Str("not verified"),
			map[string]string{"message": "You must register before you can add contacts"},
			http.StatusBadRequest))
		return
	}

	userToBeAdded, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.AddContact(userToBeAdded); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, model.MapUserToUserPartial(userToBeAdded), http.StatusCreated)
}

func (c *Config) RemoveContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := c.UserStore.GetUserByID(ctx, userID)
	if err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := u.RemoveContact(userToBeRemoved); err != nil {
		bjson.HandleError(w, err)
		return
	}

	if err := c.UserStore.Commit(ctx, u); err != nil {
		bjson.HandleError(w, err)
		return
	}

	bjson.WriteJSON(w, model.MapUserToUserPartial(userToBeRemoved), http.StatusOK)
}
