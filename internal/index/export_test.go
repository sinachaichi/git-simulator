package index

import (
	"crypto/sha1" //nolint:gosec // matches the production code
)

// WithChecksum appends a fresh checksum to body. Tests use it to build
// files that are damaged in exactly one way.
func WithChecksum(body []byte) []byte {
	sum := sha1.Sum(body) //nolint:gosec // see above
	return append(append([]byte(nil), body...), sum[:]...)
}
