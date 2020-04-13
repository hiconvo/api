package template

import (
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	"strings"
)

var templates map[string]*htmltpl.Template

func init() {
	if templates == nil {
		templates = make(map[string]*htmltpl.Template)
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	var basePath string
	if strings.HasSuffix(wd, "template") {
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
	} {
		_, ok := templates[tplName]

		if !ok {
			panic(fmt.Sprintf("Template '%v' not found", tplName))
		}
	}
}
