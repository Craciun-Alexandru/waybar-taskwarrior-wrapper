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
			text = prompt("New task › ", "")
		}
		err = add(text)
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

func printWaybar() error {
	now := time.Now()
	ts, err := pending(now)
	if err != nil {
		return err
	}
	out := map[string]string{"text": "", "class": "empty", "tooltip": "Nothing to do"}
	if len(ts) > 0 {
		top := ts[0]
		b := bucket(top, now)
		out["text"] = bucketIcon[b] + " " + html.EscapeString(strings.TrimSpace(fit(top.Summary, 30)))
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
	for b := range bucketNames {
		header := false
		for _, t := range ts {
			if bucket(t, now) != b {
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
			fmt.Fprintf(&sb, "\n<span color='%s'></span>  %s  <span color='%s'>%-14s #%s</span>",
				prioColor[t.Prio], html.EscapeString(fit(t.Summary, 36)), dimColor, dueLabel(t, now), listName(t.List))
		}
	}
	return sb.String()
}

func printList() error {
	now := time.Now()
	ts, err := pending(now)
	for _, t := range ts {
		fmt.Println(line(t, now))
	}
	return err
}

// line is the plain-text form of a task, used by `list` and the menu.
func line(t *Task, now time.Time) string {
	return fmt.Sprintf("%-3s %s  %-14s #%s", t.Prio.bangs(), fit(t.Summary, 36), dueLabel(t, now), listName(t.List))
}

func add(text string) error {
	p := parse(text, time.Now())
	if p.Summary == "" {
		return nil
	}
	if p.List == "" {
		p.List = defaultList
	}
	t := newTask(listDir(p.List))
	t.Summary, t.Due, t.HasTime, t.Prio = p.Summary, p.Due, p.HasTime, p.Prio
	return saved(t.save())
}

// doneTop completes the task shown on the bar (middle click).
func doneTop() error {
	ts, err := pending(time.Now())
	if err != nil || len(ts) == 0 {
		return err
	}
	ts[0].Done = true
	exec.Command("notify-send", "-a", "wtodo", "Done", ts[0].Summary).Run()
	return saved(ts[0].save())
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
