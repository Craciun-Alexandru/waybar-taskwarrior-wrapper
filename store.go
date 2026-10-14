package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Storage is a vdir: ~/.local/share/wtodo/<list>/<uid>.ics, one VTODO per
// file. That's what vdirsyncer syncs, so CalDAV/iPhone can be added later.

const defaultList = "personal"

// Priority: 0 none, 1 high, 2 medium, 3 low (the three levels iOS has).
type Priority int

var prioICal = [...]int{0, 1, 5, 9}

func prioFromICal(n int) Priority {
	switch {
	case n <= 0:
		return 0
	case n < 5:
		return 1
	case n == 5:
		return 2
	}
	return 3
}

// rank sorts high, medium, low, none.
func (p Priority) rank() int { return (int(p) + 3) % 4 }

// bangs is the iOS-style marker: !!! high, !! medium, ! low.
func (p Priority) bangs() string { return strings.Repeat("!", (4-int(p))%4) }

type Task struct {
	UID     string
	Parent  string // UID of the parent task, for subtasks
	Summary string
	Note    string    // DESCRIPTION; "Notes" in iOS Reminders
	Due     time.Time // zero = no due date; midnight when date-only
	HasTime bool
	Prio    Priority
	Done    bool
	List    string

	path    string
	cal     *ical.Calendar
	todo    *ical.Component
	origDue time.Time
}

func (t *Task) HasDue() bool { return !t.Due.IsZero() }

func dataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "wtodo")
	}
	return filepath.Join(os.Getenv("HOME"), ".local/share/wtodo")
}

// listName is the list's display name. vdirsyncer writes it to a
// "displayname" file (lists made on the iPhone get UUID folder names).
func listName(dir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir(), dir, "displayname"))
	if name := strings.TrimSpace(string(b)); err == nil && name != "" {
		return name
	}
	return dir
}

// listDir maps a typed "#name" to an existing folder, or a new one.
func listDir(name string) string {
	dirs, _ := filepath.Glob(filepath.Join(dataDir(), "*"))
	for _, d := range dirs {
		if id := filepath.Base(d); strings.EqualFold(id, name) || strings.EqualFold(listName(id), name) {
			return id
		}
	}
	return name
}

func loadTasks() ([]*Task, error) {
	files, err := filepath.Glob(filepath.Join(dataDir(), "*", "*.ics"))
	if err != nil {
		return nil, err
	}
	var tasks []*Task
	for _, f := range files {
		t, err := readTask(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wtodo: skipping %s: %v\n", f, err)
		} else if t != nil {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func readTask(path string) (*Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cal, err := ical.NewDecoder(f).Decode()
	if err != nil {
		return nil, err
	}
	var todo *ical.Component
	for _, c := range cal.Children {
		if c.Name == ical.CompToDo && c.Props.Get(ical.PropRecurrenceID) == nil {
			todo = c
			break
		}
	}
	if todo == nil {
		return nil, nil
	}

	p := todo.Props
	t := &Task{List: filepath.Base(filepath.Dir(path)), path: path, cal: cal, todo: todo}
	t.UID, _ = p.Text(ical.PropUID)
	for _, rel := range p.Values(ical.PropRelatedTo) {
		if rt := rel.Params.Get("RELTYPE"); rt == "" || rt == "PARENT" { // PARENT is the default
			t.Parent = rel.Value
		}
	}
	t.Summary, _ = p.Text(ical.PropSummary)
	t.Note, _ = p.Text(ical.PropDescription)
	status, _ := p.Text(ical.PropStatus)
	t.Done = status == "COMPLETED" || status == "CANCELLED" || p.Get(ical.PropCompleted) != nil
	if pr := p.Get(ical.PropPriority); pr != nil {
		n, _ := strconv.Atoi(pr.Value)
		t.Prio = prioFromICal(n)
	}
	if d := p.Get(ical.PropDue); d != nil {
		if due, err := d.DateTime(time.Local); err == nil {
			t.Due, t.HasTime = due.In(time.Local), len(d.Value) > len("20060102")
		}
	}
	t.origDue = t.Due
	return t, nil
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func newTask(list string) *Task {
	uid := newID()

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//wtodo//EN")
	todo := ical.NewComponent(ical.CompToDo)
	todo.Props.SetText(ical.PropUID, uid)
	todo.Props.SetDateTime(ical.PropCreated, time.Now().UTC())
	cal.Children = append(cal.Children, todo)
	return &Task{UID: uid, List: list, path: filepath.Join(dataDir(), list, uid+".ics"), cal: cal, todo: todo}
}

// save writes our fields into the VTODO and leaves every other property
// (e.g. iOS's X-APPLE-* ones) untouched.
func (t *Task) save() error {
	p, now := t.todo.Props, time.Now().UTC()
	p.SetText(ical.PropSummary, t.Summary)
	if t.Note = linkify(t.Note); t.Note != "" {
		p.SetText(ical.PropDescription, t.Note)
	} else {
		p.Del(ical.PropDescription)
	}

	p.Del(ical.PropPriority)
	if t.Prio != 0 {
		pr := ical.NewProp(ical.PropPriority)
		pr.Value = strconv.Itoa(prioICal[t.Prio])
		p.Set(pr)
	}

	if t.Done {
		p.SetText(ical.PropStatus, "COMPLETED")
		if p.Get(ical.PropCompleted) == nil {
			p.SetDateTime(ical.PropCompleted, now)
		}
	} else {
		p.SetText(ical.PropStatus, "NEEDS-ACTION")
		p.Del(ical.PropCompleted)
		p.Del(ical.PropPercentComplete)
	}

	switch {
	case !t.HasDue():
		p.Del(ical.PropDue)
	case t.HasTime:
		if loc := localZone(); loc != nil { // DUE;TZID=Europe/Berlin:…, like iOS
			p.SetDateTime(ical.PropDue, t.Due.In(loc))
			addTimezone(t.cal, loc, t.Due.Year())
		} else {
			p.SetDateTime(ical.PropDue, t.Due.UTC())
		}
	default:
		p.SetDate(ical.PropDue, t.Due)
	}
	if !t.Due.Equal(t.origDue) {
		// A stale start date or alarm would contradict the new due date.
		p.Del(ical.PropDateTimeStart)
		t.todo.Children = nil
		if t.HasTime { // iOS alerts only for reminders with a time
			t.todo.Children = append(t.todo.Children, alarm(t.Due))
		}
		t.origDue = t.Due
	}

	p.SetDateTime(ical.PropDateTimeStamp, now)
	p.SetDateTime(ical.PropLastModified, now)

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(t.cal); err != nil {
		return err
	}
	dir := filepath.Join(dataDir(), t.List)
	if _, err := os.Stat(dir); os.IsNotExist(err) { // new list: name it for the server
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		os.WriteFile(filepath.Join(dir, "displayname"), []byte(t.List), 0o644)
	}
	path := filepath.Join(dir, filepath.Base(t.path))
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if path != t.path { // moved to another list
		os.Remove(t.path)
		t.path = path
	}
	return nil
}

func (t *Task) remove() error { return os.Remove(t.path) }

func (t *Task) setParent(uid string) {
	rel := ical.NewProp(ical.PropRelatedTo)
	rel.Value = uid
	rel.Params.Set("RELTYPE", "PARENT")
	t.todo.Props.Set(rel)
	t.Parent = uid
}

// finish completes (or deletes) a task together with all its subtasks.
func finish(t *Task, del bool) error {
	all, err := loadTasks()
	if err != nil {
		return err
	}
	for _, x := range subtree(t, all) {
		if del {
			err = x.remove()
		} else if !x.Done {
			x.Done = true
			err = x.save()
		}
		if err != nil {
			return err
		}
	}
	return saved(nil)
}

// alarm is a phone notification at the due time, written the way iOS does.
func alarm(at time.Time) *ical.Component {
	uid := newID()
	a := ical.NewComponent(ical.CompAlarm)
	a.Props.SetText(ical.PropUID, uid)
	a.Props.SetText("X-WR-ALARMUID", uid)
	a.Props.SetText(ical.PropAction, "DISPLAY")
	a.Props.SetText(ical.PropDescription, "Reminder")
	trigger := ical.NewProp(ical.PropTrigger)
	trigger.SetDateTime(at.UTC()) // TRIGGER;VALUE=DATE-TIME:…Z
	a.Props.Set(trigger)
	return a
}
