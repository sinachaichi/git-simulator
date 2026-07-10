package porcelain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sinachaichi/git-simulator/internal/index"
	"github.com/sinachaichi/git-simulator/internal/objects"
	"github.com/sinachaichi/git-simulator/internal/repo"
	"github.com/sinachaichi/git-simulator/internal/revparse"
)

// CatFileMode selects what CatFile returns.
type CatFileMode string

// The modes of `cat-file`.
const (
	CatType   CatFileMode = "-t"
	CatSize   CatFileMode = "-s"
	CatPretty CatFileMode = "-p"
)

// CatFile shows an object: its type, its size, or its content in a
// readable form.
func CatFile(r *repo.Repo, rev string, mode CatFileMode) (string, error) {
	id, err := revparse.Resolve(r, rev)
	if err != nil {
		return "", err
	}
	o, err := r.ODB.Read(id)
	if err != nil {
		return "", err
	}
	switch mode {
	case CatType:
		return string(o.Type()) + "\n", nil
	case CatSize:
		return strconv.Itoa(len(o.Body())) + "\n", nil
	case CatPretty:
		return prettyObject(o), nil
	}
	return "", fmt.Errorf("unknown cat-file mode %q (use -t, -s or -p)", mode)
}

func prettyObject(o objects.Object) string {
	t, ok := o.(*objects.Tree)
	if !ok {
		return string(o.Body())
	}
	var b strings.Builder
	for _, e := range t.Entries {
		kind := "blob"
		if e.IsDir() {
			kind = "tree"
		}
		fmt.Fprintf(&b, "%06o %s %s\t%s\n", e.Mode, kind, e.ID, e.Name)
	}
	return b.String()
}

// HashObject returns the blob id for data and stores it when write is set.
func HashObject(r *repo.Repo, data []byte, write bool) (objects.ID, error) {
	blob := &objects.Blob{Data: data}
	if !write {
		return objects.HashOf(blob), nil
	}
	id, _, err := r.ODB.Write(blob)
	return id, err
}

// LsFiles lists the index. With stages it includes mode, id and stage
// like `ls-files --stage`.
func LsFiles(r *repo.Repo, stages bool) ([]string, error) {
	ix, err := r.LoadIndex()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ix.Entries() {
		out = append(out, formatIndexEntry(e, stages))
	}
	return out, nil
}

func formatIndexEntry(e index.Entry, stages bool) string {
	if !stages {
		return e.Path
	}
	return fmt.Sprintf("%06o %s %d\t%s", e.Mode, e.ID, e.Stage, e.Path)
}
