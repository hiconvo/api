package model

import (
	"time"
)

type Preview struct {
	Body      string       `json:"body"`
	Sender    *UserPartial `json:"user"`
	Timestamp time.Time    `json:"timestamp"`
}
