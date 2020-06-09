package template

import (
	"bytes"
	htmltpl "html/template"

	"github.com/aymerick/douceur/inliner"
	"github.com/russross/blackfriday/v2"

	"github.com/hiconvo/api/errors"
)

type renderable struct {
	RenderedBody htmltpl.HTML
}

func (r *renderable) RenderMarkdown(data string) {
	r.RenderedBody = htmltpl.HTML(blackfriday.Run([]byte(data)))
}

func (r *renderable) RenderHTML(tpl *htmltpl.Template, data interface{}) (string, error) {
	var op errors.Op = "renderable.RenderHTML"

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return "", errors.E(op, err)
	}

	html, err := inliner.Inline(buf.String())
	if err != nil {
		return html, errors.E(op, err)
	}

	return html, nil
}
