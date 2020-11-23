package model

import og "github.com/hiconvo/api/clients/opengraph"

type Preview struct {
	Body   string       `json:"body"`
	Photos []string     `json:"photos"`
	Link   *og.LinkData `json:"link"`
}
