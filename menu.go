package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Fuzzel has no modes, so vim-style NORMAL and INSERT are separate fuzzel
// runs with different key bindings. Keys bound to custom-N make fuzzel print
// the highlighted entry and exit with code 9+N.
const (
	exitDone   = 10 // l (custom-1)
	exitEdit   = 11 // h (custom-2)
	exitInsert = 12 // i (custom-3)
	exitNormal = 13 // Esc in insert mode (custom-4)
	exitSub    = 14 // o (custom-5): add a subtask
)

var navKeys = []string{
	"--override=key-bindings.next=j Down Control+n",
	"--override=key-bindings.prev=k Up Control+p",
}

func normalKeys() []string {
	// Swallow the other letters (bind them to a no-op) so they don't type.
	var rest []string
	for c := 'a'; c <= 'z'; c++ {
		if !strings.ContainsRune("jklhio", c) {
			rest = append(rest, string(c))
		}
	}
	return append(navKeys,
		"--override=key-bindings.custom-1=l",
		"--override=key-bindings.custom-2=h",
		"--override=key-bindings.custom-3=i",
		"--override=key-bindings.custom-5=o",
		"--override=key-bindings.cursor-end=End Control+e "+strings.Join(rest, " "),
		"--placeholder=i search/add · l done · h edit · o subtask · Enter actions · Esc quit")
}

var insertKeys = []string{
	"--override=key-bindings.cancel=Control+g Control+c",
	"--override=key-bindings.custom-4=Escape",
	"--placeholder=search, or type a new task · Esc normal mode",
}

// pick shows items in fuzzel and returns the chosen key (or the typed text if
// nothing matched) and fuzzel's exit code. Items are {key, label}.
func pick(promptText string, items [][2]string, extra ...string) (string, int) {
	var in strings.Builder
	for _, it := range items {
		fmt.Fprintf(&in, "%s\t%s\n", it[0], it[1])
	}
	args := append([]string{"--dmenu", "--no-sort", "--width=70", "--prompt=" + promptText,
		"--with-nth=2", "--match-nth=2", "--accept-nth=1"}, extra...)
	cmd := exec.Command("fuzzel", args...)
	cmd.Stdin = strings.NewReader(in.String())
	out, err := cmd.Output()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return strings.TrimSpace(string(out)), code
}

// prompt asks for free text, pre-filled with initial. With cursorHome the
// cursor starts at the beginning; fuzzel can't do that itself, so wtype
// presses Home once fuzzel is up (skipped if wtype isn't installed).
func prompt(promptText, initial string, cursorHome bool) string {
	if wtype, err := exec.LookPath("wtype"); err == nil && cursorHome {
		go func() {
			time.Sleep(150 * time.Millisecond)
			exec.Command(wtype, "-k", "Home").Run()
		}()
	}
	out, _ := exec.Command("fuzzel", "--dmenu", "--width=70", "--prompt-only="+promptText,
		"--search="+initial, "--mesg="+syntaxHelp).Output()
	return strings.TrimSpace(string(out))
}

func menu() error {
	insert, sel := false, 0
	for {
		now := time.Now()
		ts, err := pending(now)
		if err != nil {
			return err
		}
		nodes := tree(ts)
		items := [][2]string{{"k0", "\uf067  New task"}}
		for i, n := range nodes {
			items = append(items, [2]string{fmt.Sprint("k", i+1), line(n, now)})
		}
		sel = min(sel, len(items)-1)

		promptText, keys := "NORMAL › ", normalKeys()
		if insert {
			promptText, keys = "INSERT › ", insertKeys
		}
		key, code := pick(promptText, items, append(keys, fmt.Sprint("--select-index=", sel), "--mesg="+syntaxHelp)...)
		i := -1 // index of the chosen item; -1 for typed text
		for j, it := range items {
			if it[0] == key {
				i, sel = j, j
			}
		}

		switch {
		case code == exitInsert:
			insert = true
		case code == exitNormal:
			insert = false
		case code == exitDone && i > 0:
			err = finish(nodes[i-1].Task, false)
		case code == exitEdit && i > 0:
			err = edit(nodes[i-1].Task, now)
		case code == exitSub && i > 0:
			err = addSub(nodes[i-1].Task)
		case code == exitDone: // l on "New task": nothing to complete
		case code != 0 && code != exitEdit && code != exitSub:
			return nil // Esc in normal mode, or fuzzel closed
		case code != 0 || i == 0: // Enter, h or o on "New task"
			insert = false
			err = add(prompt("New task › ", "", false), nil)
		case i > 0:
			insert = false
			err = taskMenu(nodes[i-1].Task, now)
		case key != "": // typed text that matched nothing: add it
			insert = false
			err = add(key, nil)
		default:
			return nil
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
		{"sub", "  Add subtask"},
		{"delete", "  Delete"},
	}
	key, _ := pick(fit(t.Summary, 30)+" › ", items, append(navKeys, "--only-match")...)
	switch key {
	case "done":
		return finish(t, false)
	case "sub":
		return addSub(t)
	case "tomorrow":
		y, m, d := now.AddDate(0, 0, 1).Date()
		t.Due = time.Date(y, m, d, t.Due.Hour(), t.Due.Minute(), 0, 0, time.Local)
	case "edit":
		return edit(t, now)
	case "delete":
		return finish(t, true)
	default:
		return nil
	}
	return saved(t.save())
}

func addSub(parent *Task) error {
	return add(prompt("└─ "+strings.TrimSpace(fit(parent.Summary, 20))+" › ", "", false), parent)
}

// edit opens the task in the quick-add syntax, cursor at the start.
func edit(t *Task, now time.Time) error {
	text := prompt("Edit › ", editText(t), true)
	p := parse(text, now)
	if p.Summary == "" {
		return nil
	}
	t.Summary, t.Due, t.HasTime, t.Prio = p.Summary, p.Due, p.HasTime, p.Prio
	// The prompt shows the note on one line; only replace it if it was
	// changed, so multi-line notes from the iPhone keep their line breaks.
	if p.Note != oneLine(linkify(t.Note)) {
		t.Note = p.Note
	}
	if p.List != "" {
		t.List = listDir(p.List)
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
		parts = append(parts, "//", oneLine(linkify(t.Note)))
	}
	return strings.Join(parts, " ")
}
