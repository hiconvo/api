package handlers

import (
	"net/http"

	"github.com/getsentry/raven-go"
	"github.com/gorilla/mux"

	"github.com/hiconvo/api/middleware"
	"github.com/hiconvo/api/utils/bjson"
)

// CreateRouter mounts all of the application's endpoints. It is exported so
// that it can be used in tests.
func CreateRouter() http.Handler {
	router := mux.NewRouter()
	router.Use(raven.Recoverer)

	router.NotFoundHandler = http.HandlerFunc(notFound)
	router.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	router.HandleFunc("/inbound", Inbound).Methods("POST")

	jsonSubrouter := router.NewRoute().Subrouter()
	jsonSubrouter.Use(bjson.WithJSON, bjson.WithJSONReqBody)

	jsonSubrouter.HandleFunc("/users", CreateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/auth", AuthenticateUser).Methods("POST")
	jsonSubrouter.HandleFunc("/users/oauth", OAuth).Methods("POST")
	jsonSubrouter.HandleFunc("/users/password", UpdatePassword).Methods("POST")
	jsonSubrouter.HandleFunc("/users/verify", VerifyEmail).Methods("POST")

	authSubrouter := jsonSubrouter.NewRoute().Subrouter()
	authSubrouter.Use(middleware.WithUser)

	authSubrouter.HandleFunc("/users", GetUser).Methods("GET")
	authSubrouter.HandleFunc("/users", UpdateUser).Methods("PATCH")
	authSubrouter.HandleFunc("/users/resend", SendVerifyEmail).Methods("POST")

	authSubrouter.HandleFunc("/threads", GetThreads).Methods("GET")
	authSubrouter.HandleFunc("/threads", CreateThread).Methods("POST")
	authSubrouter.HandleFunc("/threads/{id}", GetThread).Methods("GET")
	authSubrouter.HandleFunc("/threads/{id}", UpdateThread).Methods("PATCH")
	authSubrouter.HandleFunc("/threads/{id}", DeleteThread).Methods("DELETE")

	authSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", AddUserToThread).Methods("POST")
	authSubrouter.HandleFunc("/threads/{threadID}/users/{userID}", RemoveUserFromThread).Methods("DELETE")

	authSubrouter.HandleFunc("/threads/{id}/messages", GetMessagesByThread).Methods("GET")
	authSubrouter.HandleFunc("/threads/{id}/messages", AddMessageToThread).Methods("POST")

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
