package thelpers

import (
	"os"
	"strings"
)

func IsTesting() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}
