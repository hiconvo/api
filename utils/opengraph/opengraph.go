package opengraph

import (
	"context"

	og "github.com/otiai10/opengraph"
	xurls "mvdan.cc/xurls/v2"

	"github.com/hiconvo/api/utils/reporter"
)

type LinkData struct {
	URL         string `json:"url"`
	Image       string `json:"image"`
	Favicon     string `json:"favicon"`
	Title       string `json:"title"`
	Site        string `json:"site"`
	Description string `json:"description"`
}

func Extract(ctx context.Context, text string) LinkData {
	url := xurls.Strict().FindString(text)
	if url == "" {
		return LinkData{}
	}

	data, err := og.FetchWithContext(ctx, url)
	if err != nil {
		reporter.Report(err)
		return LinkData{}
	}

	var image string
	if len(data.Image) > 0 {
		image = data.Image[0].SURL
	}

	return LinkData{
		Title:       data.Title,
		URL:         data.URL.String(),
		Site:        data.SiteName,
		Description: data.Description,
		Favicon:     data.Favicon,
		Image:       image,
	}
}
