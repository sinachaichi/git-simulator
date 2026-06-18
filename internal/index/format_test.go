package index_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/testutil"
)

func blob(s string) objects.ID { return objects.HashOf(&objects.Blob{Data: []byte(s)}) }

func sample() []index.Entry {
	when := time.Unix(1700000000, 123)
	return []index.Entry{
		{Path: "README.md", Mode: objects.ModeFile, ID: blob("a"), Size: 1, MTime: when},
		{Path: "src/main.go", Mode: objects.ModeExecutable, ID: blob("bb"), Size: 2, MTime: when},
		{Path: "src/very/long/path/that/is/not/aligned.txt", Mode: objects.ModeFile, ID: blob("c"), Size: 1, MTime: when},
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	data := index.Marshal(sample())
	got, err := index.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, sample(), got)
}

func TestMarshalPadsEntriesToEightBytes(t *testing.T) {
	for _, name := range []string{"a", "ab", "abc", "abcdefg", "abcdefgh", "abcdefghi"} {
		data := index.Marshal([]index.Entry{{Path: name, Mode: objects.ModeFile}})
		// 12 byte header + padded entry + 20 byte checksum
		assert.Zero(t, (len(data)-12-20)%8, name)
	}
}

func TestStageIsStored(t *testing.T) {
	in := []index.Entry{
		{Path: "f", Mode: objects.ModeFile, ID: blob("base"), Stage: 1},
		{Path: "f", Mode: objects.ModeFile, ID: blob("ours"), Stage: 2},
		{Path: "f", Mode: objects.ModeFile, ID: blob("theirs"), Stage: 3},
	}
	data := index.Marshal(in)
	got, err := index.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, uint8(3), got[2].Stage)
}

func TestUnmarshalDetectsDamage(t *testing.T) {
	good := index.Marshal(sample())

	flipped := append([]byte(nil), good...)
	flipped[20] ^= 0xff
	_, err := index.Unmarshal(flipped)
	assert.ErrorContains(t, err, "checksum")

	_, err = index.Unmarshal(good[:10])
	assert.ErrorContains(t, err, "too short")

	_, err = index.Unmarshal(nil)
	assert.Error(t, err)
}

func TestUnmarshalRejectsOtherVersions(t *testing.T) {
	data := index.Marshal(sample())
	data[7] = 4
	// Fix the checksum so only the version is wrong.
	data = index.WithChecksum(data[:len(data)-20])
	_, err := index.Unmarshal(data)
	assert.ErrorContains(t, err, "unsupported index version 4")
}

// Real git must accept our file, and list the same modes, ids and paths.
func TestRealGitReadsOurIndex(t *testing.T) {
	testutil.RequireGit(t)
	dir := testutil.BareRepo(t)
	data := index.Marshal(sample())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index"), data, 0o644))

	out := testutil.Git(t, dir, "", "ls-files", "--stage")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "100644 "+blob("a").String()+" 0\tREADME.md", lines[0])
	assert.Equal(t, "100755 "+blob("bb").String()+" 0\tsrc/main.go", lines[1])
	assert.Contains(t, lines[2], "src/very/long/path/that/is/not/aligned.txt")
}
