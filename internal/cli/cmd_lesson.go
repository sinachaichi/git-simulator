package cli

import (
	"fmt"

	"github.com/sinachaichi/git-simulator/internal/lessons"
)

func cmdLesson(c *Context, args []string) error {
	if len(args) == 0 {
		return usagef("lesson needs a command: list, start, check, hint, status or reset")
	}
	home, err := lessons.DefaultHome()
	if err != nil {
		return err
	}
	store := lessons.Store{Home: home}
	progress, err := store.Load()
	if err != nil {
		return fmt.Errorf("cannot read lesson progress: %w", err)
	}
	switch args[0] {
	case "list":
		return c.lessonList(progress)
	case "start":
		if len(args) != 2 {
			return usagef("lesson start needs a lesson name")
		}
		return c.lessonStart(store, progress, args[1])
	case "check", "hint", "status", "reset":
		if len(args) != 1 {
			return usagef("lesson %s takes no arguments", args[0])
		}
		return c.lessonActive(store, progress, args[0])
	}
	return usagef("unknown lesson command %q", args[0])
}

func (c *Context) lessonList(p lessons.Progress) error {
	for _, l := range lessons.All() {
		rec := p.Lessons[l.Name]
		state := c.Style.Dim("not started")
		switch {
		case rec.Done:
			state = c.Style.Green("done")
		case p.Lessons[l.Name] != (lessons.LessonRecord{}) || p.Active == l.Name:
			state = c.Style.Yellow(fmt.Sprintf("step %d of %d", rec.Step+1, len(l.Steps)))
		}
		fmt.Fprintf(c.Stdout, "  %-28s %-40s %s\n", l.Name, l.Title, state)
	}
	return nil
}

func (c *Context) lessonStart(store lessons.Store, p lessons.Progress, name string) error {
	l, err := lessons.Find(name)
	if err != nil {
		return err
	}
	if _, err := lessons.Prepare(store, l); err != nil {
		return err
	}
	p.Active = name
	p.Lessons[name] = lessons.LessonRecord{}
	if err := store.Save(p); err != nil {
		return err
	}
	workspace := store.Workspace(name)
	fmt.Fprintf(c.Stdout, "%s\n%s\n\n", c.Style.Bold("Lesson: "+l.Title), l.Summary)
	if c.ChangeDir != nil {
		c.ChangeDir(workspace)
		fmt.Fprintf(c.Stdout, "You are now in the lesson repository: %s\n\n", workspace)
	} else {
		fmt.Fprintf(c.Stdout, "The lesson repository is %s\nRun `cd %s` to work in it.\n\n", workspace, workspace)
	}
	c.printStep(l, 0)
	fmt.Fprintln(c.Stdout, "\nWhen you think you are done, run `lesson check`.")
	return nil
}

func (c *Context) printStep(l lessons.Lesson, i int) {
	fmt.Fprintf(c.Stdout, "%s %s\n", c.Style.Cyan(fmt.Sprintf("Step %d of %d:", i+1, len(l.Steps))), l.Steps[i].Task)
}

// lessonActive runs the commands that work on the active lesson.
func (c *Context) lessonActive(store lessons.Store, p lessons.Progress, sub string) error {
	if p.Active == "" {
		return fmt.Errorf("no lesson is active. Run `lesson list`, then `lesson start <name>`")
	}
	l, err := lessons.Find(p.Active)
	if err != nil {
		return err
	}
	rec := p.Lessons[l.Name]
	switch sub {
	case "reset":
		return c.lessonStart(store, p, l.Name)
	case "status":
		if rec.Done {
			fmt.Fprintf(c.Stdout, "%s is finished.\n", l.Title)
			return nil
		}
		fmt.Fprintf(c.Stdout, "%s\n", c.Style.Bold("Lesson: "+l.Title))
		c.printStep(l, rec.Step)
		return nil
	case "hint":
		if rec.Done {
			fmt.Fprintln(c.Stdout, "The lesson is finished, there is nothing left to hint at.")
			return nil
		}
		fmt.Fprintf(c.Stdout, "Hint: %s\n", l.Steps[rec.Step].Hint)
		return nil
	}

	r, err := lessons.Open(store, l.Name)
	if err != nil {
		return err
	}
	if rec.Done {
		fmt.Fprintf(c.Stdout, "%s is already finished. Start it again with `lesson start %s`.\n", l.Title, l.Name)
		return nil
	}
	out := lessons.Check(l, r, rec.Step)
	for _, n := range out.Completed {
		fmt.Fprintf(c.Stdout, "%s Step %d done.\n", c.Style.Green("ok"), n)
	}
	rec.Step += len(out.Completed)
	rec.Done = out.Done
	p.Lessons[l.Name] = rec
	if err := store.Save(p); err != nil {
		return err
	}
	if out.Done {
		fmt.Fprintf(c.Stdout, "\n%s You finished \"%s\". Try `lesson list` for the next one.\n", c.Style.Green("Well done."), l.Title)
		return nil
	}
	fmt.Fprintf(c.Stdout, "%s %s\n", c.Style.Yellow("Not yet:"), out.Missing)
	c.printStep(l, out.Next-1)
	return nil
}
