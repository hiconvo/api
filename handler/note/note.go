package note

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/model"
	"github.com/hiconvo/api/valid"
)

type Config struct {
	UserStore model.UserStore
	NoteStore model.NoteStore
	OG        opengraph.Client
}

func NewHandler(c *Config) *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.WithUser(c.UserStore))
	r.HandleFunc("/notes", c.CreateNote).Methods("POST")
	r.HandleFunc("/notes", c.GetNotes).Methods("GET")

	s := r.NewRoute().Subrouter()
	s.Use(middleware.WithNote(c.NoteStore))
	s.HandleFunc("/notes/{noteID}", c.GetNote).Methods("GET")
	s.HandleFunc("/notes/{noteID}", c.UpdateNote).Methods("PATCH")
	s.HandleFunc("/notes/{noteID}", c.DeleteNote).Methods("DELETE")

	return r
}

type createNotePayload struct {
	Name    string `validate:"max=255"`
	Favicon string
	URL     string
	Tags    []string
	Body    string
}

func (c *Config) CreateNote(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.CreateNote")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)

	var payload createNotePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	n, err := model.NewNote(
		u,
		payload.Name,
		payload.URL,
		payload.Favicon,
		payload.Body,
		payload.Tags,
	)
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := c.NoteStore.Commit(ctx, n); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, n, http.StatusCreated)
}

func (c *Config) GetNotes(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.GetNotes")
	ctx := r.Context()
	u := middleware.UserFromContext(ctx)
	p := model.GetPagination(r)

	notes, err := c.NoteStore.GetNotesByUser(ctx, u, p,
		db.GetNotesFilter(r.URL.Query().Get("filter")),
		db.GetNotesSearch(r.URL.Query().Get("search")),
		db.GetNotesTags(r.URL.Query().Get("tag")))
	if err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, map[string]interface{}{"notes": notes}, http.StatusOK)
}

func (c *Config) GetNote(w http.ResponseWriter, r *http.Request) {
	n := middleware.NoteFromContext(r.Context())
	bjson.WriteJSON(w, n, http.StatusOK)
}

type updateNotePayload struct {
	Name    string `validate:"max=255"`
	Favicon string
	URL     string
	Tags    []string
	Body    string
}

func (c *Config) UpdateNote(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.UpdateNote")
	ctx := r.Context()
	n := middleware.NoteFromContext(ctx)

	var payload updateNotePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if len(payload.Name) > 0 {
		n.Name = payload.Name
	}

	if len(payload.Favicon) > 0 {
		if url, err := valid.URL(payload.Favicon); err == nil {
			n.Favicon = url
		} else {
			bjson.HandleError(w, errors.E(op, err, http.StatusBadRequest))
			return
		}
	}

	if len(payload.URL) > 0 {
		if url, err := valid.URL(payload.URL); err == nil {
			n.URL = url
		} else {
			bjson.HandleError(w, errors.E(op, err, http.StatusBadRequest))
			return
		}
	}

	if len(payload.Body) > 0 && payload.Body != n.Body {
		n.Body = payload.Body
	}

	if err := c.NoteStore.Commit(ctx, n); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, n, http.StatusOK)
}

func (c *Config) DeleteNote(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.GetNote")
	ctx := r.Context()
	n := middleware.NoteFromContext(ctx)

	if err := c.NoteStore.Delete(ctx, n); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, n, http.StatusOK)
}
