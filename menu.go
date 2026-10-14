package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// pick shows items in fuzzel and returns the chosen key, the typed text if
// nothing matched, or "" if cancelled. Items are {key, label}.
func pick(promptText string, items [][2]string, extra ...string) string {
	var in strings.Builder
	for _, it := range items {
		fmt.Fprintf(&in, "%s\t%s\n", it[0], it[1])
	}
	args := append([]string{"--dmenu", "--no-sort", "--width=70", "--prompt=" + promptText,
		"--with-nth=2", "--match-nth=2", "--accept-nth=1"}, extra...)
	cmd := exec.Command("fuzzel", args...)
	cmd.Stdin = strings.NewReader(in.String())
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// prompt asks for free text, pre-filled with initial.
func prompt(promptText, initial string) string {
	out, _ := exec.Command("fuzzel", "--dmenu", "--width=70", "--prompt-only="+promptText,
		"--search="+initial, "--mesg="+syntaxHelp).Output()
	return strings.TrimSpace(string(out))
}

func menu() error {
	for {
		now := time.Now()
		ts, err := pending(now)
		if err != nil {
			return err
		}
		items := [][2]string{{"new", "  New task"}}
		for i, t := range ts {
			items = append(items, [2]string{fmt.Sprint("t", i), line(t, now)})
		}

		key := pick("Tasks › ", items, "--placeholder=search, or type a new task", "--mesg="+syntaxHelp)
		var i int
		switch _, scanErr := fmt.Sscanf(key, "t%d", &i); {
		case key == "":
			return nil
		case key == "new":
			err = add(prompt("New task › ", ""))
		case scanErr == nil && i < len(ts):
			err = taskMenu(ts[i], now)
		default: // typed text that matched nothing: add it
			err = add(key)
		}
		if err != nil {
			return err
		}
	}
}

func taskMenu(t *Task, now time.Time) error {
	items := [][2]string{
		{"done", "  Done"},
		{"tomorrow", "  Snooze to tomorrow"},
		{"edit", "  Edit…"},
		{"delete", "  Delete"},
	}
	switch pick(fit(t.Summary, 30)+" › ", items, "--only-match") {
	case "done":
		t.Done = true
	case "tomorrow":
		y, m, d := now.AddDate(0, 0, 1).Date()
		t.Due = time.Date(y, m, d, t.Due.Hour(), t.Due.Minute(), 0, 0, time.Local)
	case "edit":
		text := prompt("Edit › ", editText(t))
		if text == "" {
			return nil
		}
		p := parse(text, now)
		if p.Summary == "" {
			return nil
		}
		t.Summary, t.Due, t.HasTime, t.Prio = p.Summary, p.Due, p.HasTime, p.Prio
		// The prompt shows the note on one line; only replace it if it was
		// changed, so multi-line notes from the iPhone keep their line breaks.
		if p.Note != oneLine(t.Note) {
			t.Note = p.Note
		}
		if p.List != "" {
			t.List = listDir(p.List)
		}
	case "delete":
		return saved(t.remove())
	default:
		return nil
	}
	return saved(t.save())
}

// editText renders a task back into the quick-add syntax, so one prompt
// edits summary, due date, priority and list at once.
func editText(t *Task) string {
	parts := []string{t.Summary}
	if t.HasDue() {
		parts = append(parts, t.Due.Format("2006-01-02"))
		if t.HasTime {
			parts = append(parts, t.Due.Format("15:04"))
		}
	}
	if t.Prio != 0 {
		parts = append(parts, t.Prio.bangs())
	}
	parts = append(parts, "#"+t.List)
	if t.Note != "" {
		parts = append(parts, "//", oneLine(t.Note))
	}
	return strings.Join(parts, " ")
}
