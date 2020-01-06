package reporter

import (
	"log"
	"os"

	"github.com/getsentry/raven-go"
)

var _logger *log.Logger

func init() {
	_logger = log.New(os.Stderr, "ERROR: ", log.LstdFlags)
}

// Report logs the error to stderr and sends the error to Sentry
func Report(err error) {
	raven.CaptureError(err, map[string]string{"reporter": "report"})

	_logger.Println(err.Error())
}

// Log logs a message to stderr and to Sentry
func Log(s string) {
	raven.CaptureMessage(s, map[string]string{"reporter": "log"})

	_logger.Println(s)
}

// JustReport is like Report but it just logs the error to stderr
func JustReport(err error) {
	_logger.Println(err.Error())
}

// JustLog is like Log but it just logs to stderr
func JustLog(s string) {
	_logger.Println(s)
}
