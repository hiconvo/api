package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/utils/bjson"
)

// CreateRouter mounts all of the application's endpoints. It is exported so
// that it can be used in tests.
func CreateRouter() http.Handler {
	router := mux.NewRouter()
	router.Use(middleware.WithErrorReporting)

	router.NotFoundHandler = http.HandlerFunc(notFound)
	router.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	// Inbound email webhook
	router.HandleFunc("/inbound", Inbound).Methods("POST")

	// JSON endpoints
	jsonSubrouter := router.NewRoute().Subrouter()
	jsonSubrouter.Use(bjson.WithJSON, bjson.WithJSONReqBody)

	jsonSubrouter.HandleFunc("/users", CreateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/auth", AuthenticateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/oauth", OAuth).Methods("POST")
	jsonSubrouter.HandleFunc("/users/password", UpdatePassword).Methods("POST")
	jsonSubrouter.HandleFunc("/users/verify", VerifyEmail).Methods("POST")

	// JSON + Auth endpoints
	authSubrouter := jsonSubrouter.NewRoute().Subrouter()
	authSubrouter.Use(middleware.WithUser)

	authSubrouter.HandleFunc("/users", GetCurrentUser).Methods("GET")
	authSubrouter.HandleFunc("/users", UpdateUser).Methods("PATCH")
	authSubrouter.HandleFunc("/users/resend", SendVerifyEmail).Methods("POST")
	authSubrouter.HandleFunc("/users/search", UserSearch).Methods("GET")
	authSubrouter.HandleFunc("/users/avatar", PutAvatar).Methods("POST")

	authSubrouter.HandleFunc("/users/{userID}", GetUser).Methods("GET")

	authSubrouter.HandleFunc("/threads", CreateThread).Methods("POST")
	authSubrouter.HandleFunc("/threads", GetThreads).Methods("GET")

	authSubrouter.HandleFunc("/contacts", GetContacts).Methods("GET")
	authSubrouter.HandleFunc("/contacts/{userID}", AddContact).Methods("POST")
	authSubrouter.HandleFunc("/contacts/{userID}", RemoveContact).Methods("DELETE")

	// JSON + Auth + Thread endpoints
	threadSubrouter := authSubrouter.NewRoute().Subrouter()
	threadSubrouter.Use(middleware.WithThread)

	threadSubrouter.HandleFunc("/threads/{threadID}", GetThread).Methods("GET")
	threadSubrouter.HandleFunc("/threads/{threadID}", UpdateThread).Methods("PATCH")
	threadSubrouter.HandleFunc("/threads/{threadID}", DeleteThread).Methods("DELETE")

	threadSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", AddUserToThread).Methods("POST")
	threadSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", RemoveUserFromThread).Methods("DELETE")

	threadSubrouter.HandleFunc("/threads/{threadID}/messages", GetMessagesByThread).Methods("GET")
	threadSubrouter.HandleFunc("/threads/{threadID}/messages", AddMessageToThread).Methods("POST")

	return middleware.WithLogging(middleware.WithCORS(router))
}

func notFound(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{
		"message": "Not found",
	}, http.StatusNotFound)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	bjson.WriteJSON(w, map[string]string{
		"message": "Method not allowed",
	}, http.StatusMethodNotAllowed)
}
