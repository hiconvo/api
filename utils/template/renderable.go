package template

import (
	"bytes"
	htmltpl "html/template"

	"github.com/aymerick/douceur/inliner"
	"github.com/russross/blackfriday/v2"
)

type renderable struct {
	RenderedBody htmltpl.HTML
}

func (r *renderable) RenderMarkdown(data string) {
	r.RenderedBody = htmltpl.HTML(blackfriday.Run([]byte(data)))
}

func (r renderable) RenderHTML(tplName string, data interface{}) (string, error) {
	tmpl, _ := templates[tplName]

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return "", err
	}

	html, err := inliner.Inline(buf.String())
	if err != nil {
		return html, err
	}

	return html, nil
}
