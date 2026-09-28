package porcelain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sinachaichi/git-simulator/internal/porcelain"
	"github.com/sinachaichi/git-simulator/internal/repo"
)

// originalHistory rebuilds the history that the first version of this
// project used in its checkout tests: four commits that grow src/main.go.
func originalHistory(t *testing.T) *repo.Repo {
	t.Helper()
	r := newRepo(t)
	write(t, r, "README.md", "### MY GIT IMPL")
	write(t, r, "src/main.go", "")
	write(t, r, "src/workdir/file1.go", "")
	write(t, r, "src/workdir/file2.go", "")
	commitAll(t, r, "initial commit")

	require.NoError(t, r.Work.AppendFile("src/main.go", []byte("func main(){}")))
	commitAll(t, r, "feat(main)")

	require.NoError(t, r.Work.AppendFile("src/main.go", []byte("\n// modification1\n")))
	require.NoError(t, r.Work.AppendFile("src/workdir/file1.go", []byte("\n// another modification1\n")))
	commitAll(t, r, "modifications 1")

	require.NoError(t, r.Work.AppendFile("src/main.go", []byte("\n// modification2\n")))
	require.NoError(t, r.Work.AppendFile("src/workdir/file1.go", []byte("\n// another modification2\n")))
	commitAll(t, r, "modifications 2")
	return r
}

// The old tests asked for "~1", "~2", "~3" and "^", "^^", "^^^". They map to
// HEAD~N, and every one of them still works as a detached checkout.
func TestOriginalCheckoutReferences(t *testing.T) {
	cases := []struct {
		refs             []string
		hasMain          bool
		hasMod1, hasMod2 bool
		wantSubject      string
	}{
		{[]string{"HEAD~1", "HEAD^"}, true, true, false, "modifications 1"},
		{[]string{"HEAD~2", "HEAD^^"}, true, false, false, "feat(main)"},
		{[]string{"HEAD~3", "HEAD^^^"}, false, false, false, "initial commit"},
	}
	for _, c := range cases {
		for _, ref := range c.refs {
			r := originalHistory(t)
			res, err := porcelain.Switch(r, ref, porcelain.SwitchOptions{Lenient: true})
			require.NoError(t, err, ref)
			assert.True(t, res.Detached, ref)

			content := read(t, r, "src/main.go")
			assert.Equal(t, c.hasMain, strings.Contains(content, "func main(){}"), ref)
			assert.Equal(t, c.hasMod1, strings.Contains(content, "modification1"), ref)
			assert.Equal(t, c.hasMod2, strings.Contains(content, "modification2"), ref)

			head := commitOf(t, r, headID(t, r))
			assert.Equal(t, c.wantSubject, head.Subject(), ref)
		}
	}
}

func TestOriginalCheckoutCanGoBackToTheTip(t *testing.T) {
	r := originalHistory(t)
	_, err := porcelain.Switch(r, "HEAD~3", porcelain.SwitchOptions{Lenient: true})
	require.NoError(t, err)
	_, err = porcelain.Switch(r, "main", porcelain.SwitchOptions{})
	require.NoError(t, err)
	assert.Contains(t, read(t, r, "src/main.go"), "modification2")
	assert.True(t, status(t, r).Clean())
}
