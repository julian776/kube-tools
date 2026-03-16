package prometheus

import (
	"io"
	"strings"
)

// stringReader creates an io.Reader from a string, used for container file mounts.
func stringReader(s string) io.Reader {
	return strings.NewReader(s)
}
