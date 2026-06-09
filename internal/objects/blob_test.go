package objects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/objects"
)

// These ids come from real git: `printf ... | git hash-object --stdin`.
func TestBlobIDMatchesGit(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
		{"what is up, doc?", "bd9dbf5aae1a3862dd1526723246b20206e5fc37"},
	}
	for _, tt := range tests {
		got := objects.HashOf(&objects.Blob{Data: []byte(tt.content)})
		assert.Equal(t, tt.want, got.String(), "content %q", tt.content)
	}
}

func TestSerializeAddsHeader(t *testing.T) {
	got := objects.Serialize(&objects.Blob{Data: []byte("hi")})
	assert.Equal(t, "blob 2\x00hi", string(got))
}

func TestParseID(t *testing.T) {
	id, err := objects.ParseID("ce013625030ba8dba906f756967f9e9ca394464a")
	assert.NoError(t, err)
	assert.Equal(t, "ce01362", id.Short())

	for _, bad := range []string{"", "abc", "zz13625030ba8dba906f756967f9e9ca394464a", "ce013625030ba8dba906f756967f9e9ca394464a00"} {
		_, err := objects.ParseID(bad)
		assert.Error(t, err, bad)
	}
}

func TestZeroID(t *testing.T) {
	assert.True(t, objects.Zero.IsZero())
	assert.Equal(t, "0000000000000000000000000000000000000000", objects.Zero.String())
}
