package diff_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sinachaichi/git-simulator/internal/diff"
	"github.com/sinachaichi/git-simulator/internal/objects"
)

func id(s string) objects.ID { return objects.HashOf(&objects.Blob{Data: []byte(s)}) }

func short(i objects.ID) string { return i.String()[:7] }

func TestRenderModified(t *testing.T) {
	f := diff.File{
		Path: "a.txt", OldMode: objects.ModeFile, NewMode: objects.ModeFile,
		OldID: id("one\n"), NewID: id("two\n"), Old: []byte("one\n"), New: []byte("two\n"),
	}
	want := "diff --git a/a.txt b/a.txt\nindex " + short(id("one\n")) + ".." + short(id("two\n")) + " 100644\n" +
		"--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-one\n+two\n"
	assert.Equal(t, want, f.Render(short, 3))
}

func TestRenderNewAndDeleted(t *testing.T) {
	added := diff.File{Path: "n", NewMode: objects.ModeFile, NewID: id("x\n"), New: []byte("x\n")}
	assert.Equal(t, "diff --git a/n b/n\nnew file mode 100644\nindex 0000000.."+short(id("x\n"))+
		"\n--- /dev/null\n+++ b/n\n@@ -0,0 +1 @@\n+x\n", added.Render(short, 3))

	deleted := diff.File{Path: "d", OldMode: objects.ModeFile, OldID: id("x\n"), Old: []byte("x\n")}
	assert.Equal(t, "diff --git a/d b/d\ndeleted file mode 100644\nindex "+short(id("x\n"))+
		"..0000000\n--- a/d\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n", deleted.Render(short, 3))
}

func TestRenderEmptyFileAddedHasNoHunks(t *testing.T) {
	f := diff.File{Path: "e", NewMode: objects.ModeFile, NewID: id("")}
	assert.Equal(t, "diff --git a/e b/e\nnew file mode 100644\nindex 0000000..e69de29\n", f.Render(short, 3))
}

func TestRenderModeChangeOnly(t *testing.T) {
	f := diff.File{
		Path: "run.sh", OldMode: objects.ModeFile, NewMode: objects.ModeExecutable,
		OldID: id("#!\n"), NewID: id("#!\n"), Old: []byte("#!\n"), New: []byte("#!\n"),
	}
	assert.Equal(t, "diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n", f.Render(short, 3))
}

func TestRenderBinary(t *testing.T) {
	f := diff.File{Path: "img", NewMode: objects.ModeFile, NewID: id("\x00\x01"), New: []byte("\x00\x01")}
	assert.Equal(t, "diff --git a/img b/img\nnew file mode 100644\nindex 0000000.."+short(id("\x00\x01"))+
		"\nBinary files /dev/null and b/img differ\n", f.Render(short, 3))
}

func TestIsBinaryLooksOnlyAtTheStart(t *testing.T) {
	assert.True(t, diff.IsBinary([]byte("a\x00b")))
	assert.False(t, diff.IsBinary([]byte("plain text")))
	late := append([]byte(strings.Repeat("a", 8500)), 0)
	assert.False(t, diff.IsBinary(late), "git only checks the first 8000 bytes")
}
