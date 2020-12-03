package opengraph

import (
	"context"
	"net/http"
	"time"

	"github.com/dyatlov/go-htmlinfo/htmlinfo"
	xurls "mvdan.cc/xurls/v2"

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
	url := xurls.Strict().FindString(text)
	if url == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("opengraph.Extract(%s): %v", url, err)

		return nil
	}

	req.Header.Set("User-Agent", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("opengraph.Extract(%s): %v", url, err)

		return nil
	}
	defer resp.Body.Close()

	info := htmlinfo.NewHTMLInfo()

	err = info.Parse(resp.Body, &url, nil)
	if err != nil {
		log.Printf("opengraph.Extract(%s): %v", url, err)

		return nil
	}

	oembed := info.GenerateOembedFor(url)

	return &LinkData{
		Title:       oembed.Title,
		URL:         oembed.URL,
		Site:        oembed.ProviderName,
		Description: oembed.Description,
		Favicon:     info.FaviconURL,
		Image:       oembed.ThumbnailURL,
		Original:    url,
	}
}

func NewNullClient() Client {
	return &nullClient{}
}

type nullClient struct{}

func (c *nullClient) Extract(ctx context.Context, text string) *LinkData {
	return nil
}
