package models

import (
	"time"
)

type Preview struct {
	Body      string    `json:"body"`
	Sender    *Contact  `json:"sender"`
	Timestamp time.Time `json:"timestamp"`
}
