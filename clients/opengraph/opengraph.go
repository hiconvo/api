package opengraph

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dyatlov/go-htmlinfo/htmlinfo"
	xurls "mvdan.cc/xurls/v2"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

type LinkData struct {
	URL         string `json:"url"`
	Image       string `json:"image" datastore:",noindex"`
	Favicon     string `json:"favicon" datastore:",noindex"`
	Title       string `json:"title"`
	Site        string `json:"site"`
	Description string `json:"description" datastore:",noindex"`
	Original    string `json:"-"`
	HTML        string `json:"html"`
}

type Client interface {
	Extract(ctx context.Context, text string) *LinkData
}

func NewClient() Client {
	return &clientImpl{
		httpClient: &http.Client{Timeout: time.Duration(5) * time.Second},
	}
}

type clientImpl struct {
	httpClient *http.Client
}

func (c *clientImpl) Extract(ctx context.Context, text string) *LinkData {
	found := xurls.Strict().FindString(text)
	if found == "" {
		return nil
	}

	op := errors.Opf("opengraph.Extract(%s)", found)

	urlobj, err := url.ParseRequestURI(found)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	if hn := urlobj.Hostname(); hn == "youtu.be" || strings.HasSuffix(hn, "youtube.com") {
		return c.handleYouTube(ctx, urlobj.String(), found)
	} else if strings.HasSuffix(hn, "twitter.com") {
		return c.handleTwitter(ctx, urlobj.String(), found)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, found, nil)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	req.Header.Set("User-Agent", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}
	defer resp.Body.Close()

	info := htmlinfo.NewHTMLInfo()

	err = info.Parse(resp.Body, &found, nil)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	oembed := info.GenerateOembedFor(found)

	return &LinkData{
		Title:       oembed.Title,
		URL:         oembed.URL,
		Site:        oembed.ProviderName,
		Description: info.OGInfo.Description,
		Favicon:     info.FaviconURL,
		Image:       oembed.ThumbnailURL,
		Original:    found,
	}
}

func (c *clientImpl) handleYouTube(ctx context.Context, found, original string) *LinkData {
	purl := fmt.Sprintf("https://www.youtube.com/oembed?url=%s&format=json", html.EscapeString(found))
	op := errors.Opf("opengraph.handleYouTube(%s)", purl)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, purl, nil)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Print(errors.E(op, errors.Errorf("received status code %d", resp.StatusCode)))

		return nil
	}

	var msi map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&msi)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	ld := &LinkData{
		Site:        "YouTube",
		Favicon:     "https://www.youtube.com/favicon.ico",
		URL:         found,
		Original:    original,
		Description: "",
	}

	title, ok := msi["title"].(string)
	if !ok {
		log.Print(errors.E(op, errors.Str("no title in response")))

		return nil
	} else {
		ld.Title = title
	}

	thumbnail, ok := msi["thumbnail_url"].(string)
	if !ok {
		log.Print(errors.E(op, errors.Str("no thumbnail in response")))
	} else {
		ld.Image = thumbnail
	}

	html, ok := msi["html"].(string)
	if !ok {
		log.Print(errors.E(op, errors.Str("no html in response")))
	} else {
		ld.HTML = html
	}

	return ld
}

func (c *clientImpl) handleTwitter(ctx context.Context, found, original string) *LinkData {
	purl := fmt.Sprintf("https://publish.twitter.com/oembed?url=%s&format=json", html.EscapeString(found))
	op := errors.Opf("opengraph.handleTwitter(%s)", purl)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, purl, nil)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Print(errors.E(op, err))

		return nil
	}

	var msi map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&msi)
	if err != nil {
		log.Print(errors.E(op, err))

		return nil
	}

	ld := &LinkData{
		Site:        "Twitter",
		Favicon:     "https://www.twitter.com/favicon.ico",
		URL:         found,
		Original:    original,
		Description: "",
	}

	html, ok := msi["html"].(string)
	if !ok {
		log.Print(errors.E(op, err))

		return nil
	} else {
		ld.HTML = html
	}

	return ld
}

func NewNullClient() Client {
	return &nullClient{}
}

type nullClient struct{}

func (c *nullClient) Extract(ctx context.Context, text string) *LinkData {
	return nil
}
