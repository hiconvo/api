package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/models"
	"github.com/hiconvo/api/utils/bjson"
)

var (
	errMsgGetContact      = map[string]string{"message": "Could not find contact"}
	errMsgAddContact      = map[string]string{"message": "Could not add contact"}
	errMsgAddSelf         = map[string]string{"message": "Cannot add self as contact"}
	errMsgHasContact      = map[string]string{"message": "You already have this contact"}
	errMsgTooManyContacts = map[string]string{"message": "You cannot have more than 50 contacts"}
)

func GetContacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	contacts, err := models.GetContactsByUser(ctx, &u)
	if err != nil {
		bjson.HandleInternalServerError(w, err, errMsgGetContact)
		return
	}

	bjson.WriteJSON(w, map[string][]*models.UserPartial{"contacts": contacts}, http.StatusOK)
}

func AddContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeAdded, err := models.GetUserByID(ctx, userID)
	if err != nil {
		bjson.WriteJSON(w, errMsgGetContact, http.StatusNotFound)
		return
	}

	if u.HasContact(&userToBeAdded) {
		bjson.WriteJSON(w, errMsgHasContact, http.StatusBadRequest)
		return
	}

	if u.Key.Equal(userToBeAdded.Key) {
		bjson.WriteJSON(w, errMsgAddSelf, http.StatusBadRequest)
		return
	}

	if len(u.ContactKeys) >= 50 {
		bjson.WriteJSON(w, errMsgTooManyContacts, http.StatusBadRequest)
		return
	}

	u.AddContact(&userToBeAdded)

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgAddContact)
		return
	}

	bjson.WriteJSON(w, models.MapUserToUserPartial(&userToBeAdded), http.StatusCreated)
}

func RemoveContact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	vars := mux.Vars(r)
	userID := vars["userID"]

	userToBeRemoved, err := models.GetUserByID(ctx, userID)
	if err != nil {
		bjson.WriteJSON(w, errMsgGetContact, http.StatusNotFound)
		return
	}

	if !u.HasContact(&userToBeRemoved) {
		bjson.WriteJSON(w, errMsgGetContact, http.StatusBadRequest)
		return
	}

	u.RemoveContact(&userToBeRemoved)

	if err := u.Commit(ctx); err != nil {
		bjson.HandleInternalServerError(w, err, errMsgAddContact)
		return
	}

	bjson.WriteJSON(w, models.MapUserToUserPartial(&userToBeRemoved), http.StatusOK)
}
