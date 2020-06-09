package template

import (
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/hiconvo/api/clients/opengraph"
)

const (
	_tplStrMessage      = "%s said:\n\n%s\n\n"
	_tplStrEvent        = "%s invited you to:\n\n%s\n\n%s\n\n%s\n\n%s\n"
	_tplStrCancellation = "%s has cancelled:\n\n%s\n\n%s\n\n%s\n\n%s"
)

// Message is a renderable message. It is always a constituent of a
// Thread. The Body field accepts markdown. XML is not allowed.
type Message struct {
	renderable
	Body      string
	Name      string
	FromID    string
	ToID      string
	HasPhoto  bool
	HasLink   bool
	Link      *opengraph.LinkData
	MagicLink string
}

// Thread is a representation of a renderable email thread.
type Thread struct {
	renderable
	Subject   string
	FromName  string
	Messages  []Message
	Preview   string
	MagicLink string
}

// Event is a representation of a renderable email event.
type Event struct {
	renderable
	Name        string
	Address     string
	Time        string
	Description string
	Preview     string
	FromName    string
	MagicLink   string
	ButtonText  string
	Message     string
}

// Digest is a representation of a renderable email digest.
type Digest struct {
	renderable
	Items     []Thread
	Preview   string
	Events    []Event
	MagicLink string
}

// AdminEmail is a representation of a renderable administrative
// email which includes a call to action button.
type AdminEmail struct {
	renderable
	Body       string
	ButtonText string
	MagicLink  string
	Fargs      []interface{}
	Preview    string
}

type Client struct {
	templates map[string]*htmltpl.Template
}

func NewClient() *Client {
	templates := make(map[string]*htmltpl.Template)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	var basePath string
	if strings.HasSuffix(wd, "template") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// layouts and includes dirs consistently.
		basePath = "../template"
	} else {
		basePath = "./template"
	}

	layouts, err := filepath.Glob(basePath + "/layouts/*.html")
	if err != nil {
		panic(err)
	}

	includes, err := filepath.Glob(basePath + "/includes/*.html")
	if err != nil {
		panic(err)
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range layouts {
		files := append(includes, layout)
		templates[filepath.Base(layout)] = htmltpl.Must(htmltpl.ParseFiles(files...))
	}

	// Make sure the expected templates are there
	for _, tplName := range []string{
		"thread.html",
		"event.html",
		"cancellation.html",
		"digest.html",
		"admin.html",
	} {
		_, ok := templates[tplName]

		if !ok {
			panic(fmt.Sprintf("Template '%v' not found", tplName))
		}
	}

	return &Client{
		templates: templates,
	}
}

// RenderThread returns a rendered thread email.
func (c *Client) RenderThread(t *Thread) (string, string, error) {
	var builder strings.Builder

	for i, m := range t.Messages {
		fmt.Fprintf(&builder, _tplStrMessage, m.Name, m.Body)
		t.Messages[i].RenderMarkdown(t.Messages[i].Body)
	}

	plainText := builder.String()
	preview := getPreview(plainText)

	t.Preview = preview

	html, err := t.RenderHTML(c.templates["thread.html"], t)

	return plainText, html, err
}

// RenderEvent returns a rendered event invitation email.
func (c *Client) RenderEvent(e *Event) (string, string, error) {
	e.RenderMarkdown(e.Description)

	var builder strings.Builder
	fmt.Fprintf(&builder, _tplStrEvent,
		e.FromName,
		e.Name,
		e.Address,
		e.Time,
		e.Description)
	plainText := builder.String()
	preview := getPreview(plainText)

	e.Preview = preview

	html, err := e.RenderHTML(c.templates["event.html"], e)

	return plainText, html, err
}

// RenderCancellation returns a rendered event cancellation email.
func (c *Client) RenderCancellation(e *Event) (string, string, error) {
	e.RenderMarkdown(e.Message)

	var builder strings.Builder
	fmt.Fprintf(&builder, _tplStrCancellation,
		e.FromName,
		e.Name,
		e.Address,
		e.Time,
		e.Message)
	plainText := builder.String()
	preview := getPreview(plainText)

	e.Preview = preview

	html, err := e.RenderHTML(c.templates["cancellation.html"], e)

	return plainText, html, err
}

// RenderDigest returns a rendered digest email.
func (c *Client) RenderDigest(d *Digest) (string, string, error) {
	for i := range d.Items {
		for j := range d.Items[i].Messages {
			d.Items[i].Messages[j].RenderMarkdown(d.Items[i].Messages[j].Body)
		}
	}

	plainText := "You have notifications on Convo."
	d.Preview = plainText

	html, err := d.RenderHTML(c.templates["digest.html"], d)

	return plainText, html, err
}

// RenderAdminEmail returns a rendered admin email.
func (c *Client) RenderAdminEmail(a *AdminEmail) (string, string, error) {
	var builder strings.Builder
	fmt.Fprintf(&builder, a.Body, a.Fargs...)
	plainText := builder.String()
	preview := getPreview(plainText)

	a.Preview = preview

	a.RenderMarkdown(plainText)
	html, err := a.RenderHTML(c.templates["admin.html"], a)

	return plainText, html, err
}

func getPreview(plainText string) string {
	if len(plainText) > 200 {
		return plainText[:200] + "..."
	}

	return plainText
}
