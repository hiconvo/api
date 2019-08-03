package models

import (
	"time"
)

type Preview struct {
	Body      string       `json:"body"`
	Sender    *UserPartial `json:"sender"`
	Timestamp time.Time    `json:"timestamp"`
}
