// Package lessons has small guided exercises. Each one prepares a repo,
// then checks the state of that repo step by step. Nothing is judged by
// the commands typed, only by what the repository looks like.
package lessons

import (
	"fmt"
	"sort"

	"github.com/sinachaichi/git-simulator/internal/repo"
)

// Step is one task of a lesson.
type Step struct {
	Task string // what to do, shown when the step starts
	Hint string // an extra nudge, shown on request
	// Check looks at the repo. It returns true when the task is done, and
	// otherwise a short sentence about what is still missing.
	Check func(r *repo.Repo) (done bool, missing string)
}

// Lesson is a named exercise.
type Lesson struct {
	Name    string
	Title   string
	Summary string
	// Setup prepares the workspace repo, which is empty and has a user
	// name and email.
	Setup func(r *repo.Repo) error
	Steps []Step
}

// All returns every lesson, in the order they should be taken.
func All() []Lesson { return catalog() }

// Find returns the lesson with the given name.
func Find(name string) (Lesson, error) {
	for _, l := range catalog() {
		if l.Name == name {
			return l, nil
		}
	}
	names := make([]string, 0, len(catalog()))
	for _, l := range catalog() {
		names = append(names, l.Name)
	}
	sort.Strings(names)
	return Lesson{}, fmt.Errorf("no lesson called %q (known: %v)", name, names)
}

// Outcome is the result of checking a lesson.
type Outcome struct {
	Completed []int  // steps that were finished by this check, 1-based
	Done      bool   // the whole lesson is finished
	Next      int    // the step that is still open, 1-based, 0 when done
	Missing   string // what the open step still needs
}

// Check runs the steps from `from` (0-based) on. It stops at the first one
// that is not done yet.
func Check(l Lesson, r *repo.Repo, from int) Outcome {
	var out Outcome
	for i := from; i < len(l.Steps); i++ {
		done, missing := l.Steps[i].Check(r)
		if !done {
			out.Next, out.Missing = i+1, missing
			return out
		}
		out.Completed = append(out.Completed, i+1)
	}
	out.Done = true
	return out
}
