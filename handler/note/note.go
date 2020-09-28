package note

import (
	"html"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/hiconvo/api/bjson"
	"github.com/hiconvo/api/clients/opengraph"
	"github.com/hiconvo/api/db"
	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/handler/middleware"
	"github.com/hiconvo/api/log"
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
	Favicon string `validate:"max=1023"`
	URL     string `validate:"max=1023"`
	Tags    []string
	Body    string `validate:"max=3071"`
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
		html.UnescapeString(payload.Name),
		payload.URL,
		payload.Favicon,
		html.UnescapeString(payload.Body),
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

	resp := map[string]interface{}{"notes": notes}

	if p.Page == 0 {
		pins, err := c.NoteStore.GetNotesByUser(ctx, u,
			&model.Pagination{Size: -1}, db.GetNotesPins())
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}

		resp["pins"] = pins
	}

	bjson.WriteJSON(w, resp, http.StatusOK)
}

func (c *Config) GetNote(w http.ResponseWriter, r *http.Request) {
	n := middleware.NoteFromContext(r.Context())
	bjson.WriteJSON(w, n, http.StatusOK)
}

type updateNotePayload struct {
	Name    string `validate:"max=255"`
	Favicon string `validate:"max=1023"`
	URL     string `validate:"max=1023"`
	Tags    *[]string
	Body    *string `validate:"max=3071"`
	Pin     *bool
}

func (c *Config) UpdateNote(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.UpdateNote")
	ctx := r.Context()
	n := middleware.NoteFromContext(ctx)
	u := middleware.UserFromContext(ctx)

	var payload updateNotePayload
	if err := bjson.ReadJSON(&payload, r); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if err := valid.Raw(&payload); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	if name := html.UnescapeString(payload.Name); len(name) > 0 {
		n.Name = name
	}

	if len(payload.Favicon) > 0 {
		if url, err := valid.URL(payload.Favicon); err == nil {
			n.Favicon = url
		} else {
			bjson.HandleError(w, errors.E(op, err, http.StatusBadRequest))
			return
		}
	}

	if n.Variant == "note" && len(payload.URL) > 0 {
		bjson.HandleError(w, errors.E(op,
			errors.Str("url cannot be added to note"),
			map[string]string{"message": "URL cannot be added to note of variant note"},
			http.StatusBadRequest))
		return
	}

	if len(payload.URL) > 0 {
		if url, err := valid.URL(payload.URL); err == nil {
			n.URL = url
		} else {
			bjson.HandleError(w, errors.E(op, err, http.StatusBadRequest))
			return
		}
	}

	if payload.Body != nil {
		if body := html.UnescapeString(*payload.Body); body != n.Body {
			n.Body = body

			if n.Variant == "note" {
				n.UpdateNameFromBody(body)
			}
		}
	}

	if payload.Pin != nil && *payload.Pin != n.Pin {
		n.Pin = *payload.Pin
	}

	if payload.Tags != nil {
		userChanged, err := model.TabulateNoteTags(u, n, *payload.Tags)
		if err != nil {
			bjson.HandleError(w, errors.E(op, err))
			return
		}

		if userChanged {
			if err := c.UserStore.Commit(ctx, u); err != nil {
				bjson.HandleError(w, errors.E(op, err))
				return
			}
		}
	}

	if err := c.NoteStore.Commit(ctx, n); err != nil {
		log.Alarm(errors.Errorf("Inconsistent data detected for u=%s note=%v", u.Email, n.Key.ID))
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, n, http.StatusOK)
}

func (c *Config) DeleteNote(w http.ResponseWriter, r *http.Request) {
	op := errors.Op("handlers.DeleteNote")
	ctx := r.Context()
	n := middleware.NoteFromContext(ctx)

	if err := c.NoteStore.Delete(ctx, n); err != nil {
		bjson.HandleError(w, errors.E(op, err))
		return
	}

	bjson.WriteJSON(w, n, http.StatusOK)
}
