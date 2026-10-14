package main

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Must match "signal" in the waybar module config.
const waybarSignal = "-RTMIN+8"

// Tooltip colours, from the matugen palette. Index = Priority.
var prioColor = [...]string{"#8a9296", "#ffb4ab", "#f9e2af", "#88d1ec"}

const dimColor = "#8a9296"

// Due buckets, used for tooltip sections, bar icon and CSS class.
var (
	bucketNames = [...]string{"Overdue", "Today", "Tomorrow", "Later", "No date"}
	bucketClass = [...]string{"overdue", "today", "tomorrow", "later", "nodate"}
	bucketIcon  = [...]string{"", "", "", "", ""}
)

func main() {
	cmd := "waybar"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "waybar":
		err = printWaybar()
	case "menu":
		err = menu()
	case "add":
		text := strings.Join(os.Args[2:], " ")
		if text == "" {
			text = prompt("New task › ", "", false)
		}
		err = add(text, nil)
	case "done":
		err = doneTop()
	case "list":
		err = printList()
	default:
		fmt.Fprintln(os.Stderr, "usage: wtodo [waybar | menu | add [text] | done | list]")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "wtodo:", err)
		os.Exit(1)
	}
}

// pending returns open tasks, most pressing first: anything due by the end
// of today, then by priority, then by due date (undated last).
func pending(now time.Time) ([]*Task, error) {
	all, err := loadTasks()
	var ts []*Task
	for _, t := range all {
		if !t.Done {
			ts = append(ts, t)
		}
	}
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if ua, ub := bucket(a, now) <= 1, bucket(b, now) <= 1; ua != ub {
			return ua
		}
		if a.Prio != b.Prio {
			return a.Prio.rank() < b.Prio.rank()
		}
		if a.HasDue() != b.HasDue() {
			return a.HasDue()
		}
		return a.Due.Before(b.Due)
	})
	return ts, err
}

// days counts calendar days from now until t (negative if in the past).
func days(now, t time.Time) int {
	return int(math.Round(startOfDay(t).Sub(startOfDay(now)).Hours() / 24))
}

func bucket(t *Task, now time.Time) int {
	if !t.HasDue() {
		return 4
	}
	d := days(now, t.Due)
	switch {
	case d < 0 || t.HasTime && t.Due.Before(now):
		return 0
	case d == 0:
		return 1
	case d == 1:
		return 2
	}
	return 3
}

func dueLabel(t *Task, now time.Time) string {
	if !t.HasDue() {
		return ""
	}
	clock := ""
	if t.HasTime {
		clock = t.Due.Format(" 15:04")
	}
	switch d := days(now, t.Due); {
	case d < 0:
		return fmt.Sprintf("%dd late", -d)
	case d == 0 && clock != "":
		return clock[1:]
	case d == 0:
		return "today"
	case d == 1:
		return "tomorrow" + clock
	case d < 7:
		return t.Due.Format("Mon") + clock
	}
	return t.Due.Format("Jan 2") + clock
}

// fit truncates or pads s to exactly n characters.
func fit(s string, n int) string {
	if utf8.RuneCountInString(s) > n {
		return string([]rune(s)[:n-1]) + "…"
	}
	return s + strings.Repeat(" ", n-utf8.RuneCountInString(s))
}

// oneLine collapses newlines and runs of spaces into single spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func printWaybar() error {
	now := time.Now()
	ts, err := pending(now)
	if err != nil {
		return err
	}
	out := map[string]string{"text": "", "class": "empty", "tooltip": "Nothing to do"}
	if len(ts) > 0 {
		top, name := ts[0], ts[0].Summary
		for _, p := range ts { // show a subtask as "parent › subtask"
			if p.UID == top.Parent {
				name = strings.TrimSpace(fit(p.Summary, 14)) + " › " + name
			}
		}
		b := bucket(top, now)
		out["text"] = bucketIcon[b] + " " + html.EscapeString(strings.TrimSpace(fit(name, 36)))
		if l := dueLabel(top, now); l != "" {
			out["text"] += " <span alpha='60%'>· " + l + "</span>"
		}
		out["class"] = bucketClass[b]
		out["tooltip"] = tooltip(ts, now)
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func tooltip(ts []*Task, now time.Time) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<b>%d open</b>", len(ts))
	nodes := tree(ts)
	// Each top-level task goes in the section of its most urgent subtask,
	// and its whole subtree is shown under it.
	for b := range bucketNames {
		header, in := false, false
		for _, n := range nodes {
			if n.pre == "" {
				in = bucket(n.top, now) == b
			}
			if !in {
				continue
			}
			if !header {
				color := "#88d1ec"
				if b == 0 {
					color = prioColor[1]
				}
				fmt.Fprintf(&sb, "\n\n<span color='%s'><b>%s</b></span>", color, bucketNames[b])
				header = true
			}
			fmt.Fprintf(&sb, "\n<span color='%s'>\uf111</span>  <span color='%s'>%s</span>%s  <span color='%s'>%-14s #%s</span>",
				prioColor[n.Prio], dimColor, n.pre, html.EscapeString(fit(n.Summary, 36-len([]rune(n.pre)))),
				dimColor, dueLabel(n.Task, now), listName(n.List))
			indent := n.cont // notes line up with the summary; keep the branch to subtasks going
			if n.kids {
				indent += "│  "
			}
			for _, l := range noteLines(n.Note, max(20, 52-len([]rune(indent)))) {
				fmt.Fprintf(&sb, "\n   <span color='%s'>%s<i>%s</i></span>", dimColor, indent, l)
			}
		}
	}
	return sb.String()
}

func printList() error {
	now := time.Now()
	ts, err := pending(now)
	for _, n := range tree(ts) {
		fmt.Println(line(n, now))
	}
	return err
}

// line is the plain-text form of a task, used by `list` and the menu.
func line(n node, now time.Time) string {
	return fmt.Sprintf("%-3s %s  %-14s #%s", n.Prio.bangs(), fit(n.pre+n.Summary, 36), dueLabel(n.Task, now), listName(n.List))
}

// add creates a task from quick-add text; with a parent, a subtask in the
// parent's list.
func add(text string, parent *Task) error {
	p := parse(text, time.Now())
	if p.Summary == "" {
		return nil
	}
	list := listDir(orDefault(p.List, defaultList))
	if parent != nil {
		list = parent.List
	}
	t := newTask(list)
	t.Summary, t.Due, t.HasTime, t.Prio, t.Note = p.Summary, p.Due, p.HasTime, p.Prio, p.Note
	if parent != nil {
		t.setParent(parent.UID)
	}
	return saved(t.save())
}

// doneTop completes the task shown on the bar (middle click).
func doneTop() error {
	ts, err := pending(time.Now())
	if err != nil || len(ts) == 0 {
		return err
	}
	exec.Command("notify-send", "-a", "wtodo", "Done", ts[0].Summary).Run()
	return finish(ts[0], false)
}

// saved refreshes the bar after a successful write.
func saved(err error) error {
	if err == nil {
		exec.Command("pkill", waybarSignal, "waybar").Run()
		// Push to the phone right away; harmless if sync isn't set up.
		exec.Command("systemctl", "--user", "start", "--no-block", "wtodo-sync.service").Run()
	}
	return err
}
