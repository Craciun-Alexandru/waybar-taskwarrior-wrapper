package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func encoded(task *Task) string {
	var buf bytes.Buffer
	ical.NewEncoder(&buf).Encode(task.cal)
	return buf.String()
}

func TestAlarm(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	task := newTask("personal")
	task.Summary = "x"
	task.Due, task.HasTime = time.Date(2026, 10, 13, 20, 58, 0, 0, time.UTC), true
	task.save()
	if s := encoded(task); !strings.Contains(s, "TRIGGER;VALUE=DATE-TIME:20261013T205800Z") {
		t.Fatalf("no alarm at due time:\n%s", s)
	}

	task.Due = task.Due.Add(time.Hour) // snooze: the old alarm is replaced
	task.save()
	s := encoded(task)
	if strings.Count(s, "BEGIN:VALARM") != 1 || !strings.Contains(s, "DATE-TIME:20261013T215800Z") {
		t.Fatalf("alarm not moved:\n%s", s)
	}

	uid, _ := task.todo.Children[0].Props.Text(ical.PropUID)
	task.Summary = "renamed" // due unchanged: the same alarm is kept
	task.save()
	if got, _ := task.todo.Children[0].Props.Text(ical.PropUID); len(task.todo.Children) != 1 || got != uid {
		t.Fatal("alarm changed although the due date didn't")
	}

	task.HasTime = false // date-only reminders get no alert
	task.Due = time.Date(2026, 10, 14, 0, 0, 0, 0, time.Local)
	task.save()
	if strings.Contains(encoded(task), "VALARM") {
		t.Fatal("date-only task has an alarm")
	}
}
