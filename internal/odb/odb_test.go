package odb_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/odb"
)

func newStore(t *testing.T) *odb.Store {
	t.Helper()
	s, err := odb.New(filepath.Join(t.TempDir(), "objects"))
	require.NoError(t, err)
	return s
}

func TestWriteThenRead(t *testing.T) {
	s := newStore(t)
	id, created, err := s.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "ce013625030ba8dba906f756967f9e9ca394464a", id.String())

	got, err := s.Read(id)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello\n"), got.(*objects.Blob).Data)
}

func TestWriteUsesFanOutLayout(t *testing.T) {
	s := newStore(t)
	_, _, err := s.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(s.Dir, "ce", "013625030ba8dba906f756967f9e9ca394464a"))
}

func TestWriteTwiceDoesNotCreateAgain(t *testing.T) {
	s := newStore(t)
	_, created, _ := s.Write(&objects.Blob{Data: []byte("same")})
	assert.True(t, created)
	_, created, err := s.Write(&objects.Blob{Data: []byte("same")})
	require.NoError(t, err)
	assert.False(t, created)
}

func TestReadMissing(t *testing.T) {
	s := newStore(t)
	_, err := s.Read(objects.HashOf(&objects.Blob{Data: []byte("nope")}))
	assert.True(t, errors.Is(err, odb.ErrNotFound))
}

func TestReadDetectsCorruption(t *testing.T) {
	s := newStore(t)
	id, _, err := s.Write(&objects.Blob{Data: []byte("hello\n")})
	require.NoError(t, err)
	p := filepath.Join(s.Dir, id.String()[:2], id.String()[2:])
	require.NoError(t, os.Chmod(p, 0o644))
	require.NoError(t, os.WriteFile(p, []byte("not zlib"), 0o644))
	_, err = s.Read(id)
	assert.ErrorContains(t, err, "corrupt")
}

func TestReadDetectsWrongContent(t *testing.T) {
	s := newStore(t)
	a, _, _ := s.Write(&objects.Blob{Data: []byte("a")})
	b, _, _ := s.Write(&objects.Blob{Data: []byte("b")})
	pa := filepath.Join(s.Dir, a.String()[:2], a.String()[2:])
	pb := filepath.Join(s.Dir, b.String()[:2], b.String()[2:])
	data, err := os.ReadFile(pb)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(pa, 0o644))
	require.NoError(t, os.WriteFile(pa, data, 0o644))
	_, err = s.Read(a)
	assert.ErrorContains(t, err, "hashes to")
}

func TestResolvePrefix(t *testing.T) {
	s := newStore(t)
	id, _, _ := s.Write(&objects.Blob{Data: []byte("hello\n")})

	got, err := s.Resolve("ce0136")
	require.NoError(t, err)
	assert.Equal(t, id, got)

	got, err = s.Resolve(id.String())
	require.NoError(t, err)
	assert.Equal(t, id, got)

	_, err = s.Resolve("ffff")
	assert.True(t, errors.Is(err, odb.ErrNotFound))
	for _, bad := range []string{"ce", "xyz1234", ""} {
		_, err = s.Resolve(bad)
		assert.Error(t, err, bad)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	s := newStore(t)
	// Write blobs until two share the same first 4 hex characters.
	seen := map[string]bool{}
	for i := 0; i < 200000; i++ {
		id, _, err := s.Write(&objects.Blob{Data: []byte{byte(i), byte(i >> 8), byte(i >> 16)}})
		require.NoError(t, err)
		if seen[id.String()[:4]] {
			_, err := s.Resolve(id.String()[:4])
			var amb *odb.AmbiguousError
			require.True(t, errors.As(err, &amb))
			assert.Len(t, amb.Candidates, 2)
			return
		}
		seen[id.String()[:4]] = true
	}
	t.Fatal("no collision found")
}

func TestTypedReaders(t *testing.T) {
	s := newStore(t)
	blobID, _, _ := s.Write(&objects.Blob{Data: []byte("x")})
	treeID, _, _ := s.Write(&objects.Tree{})

	_, err := s.ReadBlob(blobID)
	assert.NoError(t, err)
	_, err = s.ReadTree(treeID)
	assert.NoError(t, err)

	_, err = s.ReadTree(blobID)
	assert.ErrorContains(t, err, "is a blob, not a tree")
	_, err = s.ReadCommit(treeID)
	assert.ErrorContains(t, err, "is a tree, not a commit")
	_, err = s.ReadBlob(treeID)
	assert.ErrorContains(t, err, "is a tree, not a blob")
}
