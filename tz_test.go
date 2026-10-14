package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func TestVTimezone(t *testing.T) {
	cases := map[string][]string{
		"America/Chicago": { // same rules iOS wrote for your zone
			"DAYLIGHT DTSTART:20260308T020000 RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU TZNAME:CDT TZOFFSETFROM:-0600 TZOFFSETTO:-0500",
			"STANDARD DTSTART:20261101T020000 RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU TZNAME:CST TZOFFSETFROM:-0500 TZOFFSETTO:-0600",
		},
		"Europe/Berlin": {
			"DAYLIGHT DTSTART:20260329T020000 RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU TZNAME:CEST TZOFFSETFROM:+0100 TZOFFSETTO:+0200",
			"STANDARD DTSTART:20261025T030000 RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU TZNAME:CET TZOFFSETFROM:+0200 TZOFFSETTO:+0100",
		},
		"Asia/Tokyo": {"STANDARD DTSTART:19700101T000000 TZNAME:JST TZOFFSETFROM:+0900 TZOFFSETTO:+0900"},
	}
	for name, want := range cases {
		loc, _ := time.LoadLocation(name)
		cal := ical.NewCalendar()
		addTimezone(cal, loc, 2026)
		addTimezone(cal, loc, 2026) // no duplicate
		if len(cal.Children) != 1 {
			t.Fatalf("%s: %d VTIMEZONEs", name, len(cal.Children))
		}
		var got []string
		for _, r := range cal.Children[0].Children {
			line := r.Name
			for _, k := range []string{"DTSTART", "RRULE", "TZNAME", "TZOFFSETFROM", "TZOFFSETTO"} {
				if p := r.Props.Get(k); p != nil {
					line += " " + k + ":" + p.Value
				}
			}
			got = append(got, line)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s:\n%s", name, strings.Join(got, "\n"))
		}
	}
}

func TestDueRoundTrip(t *testing.T) {
	t.Setenv("TZ", "America/Chicago")
	loc, _ := time.LoadLocation("America/Chicago")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	task := newTask("personal")
	task.Summary, task.Due, task.HasTime = "x", time.Date(2026, 10, 13, 20, 58, 0, 0, loc), true
	if err := task.save(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	ical.NewEncoder(&buf).Encode(task.cal)
	if !strings.Contains(buf.String(), "DUE;TZID=America/Chicago:20261013T205800") {
		t.Errorf("DUE not in local zone:\n%s", buf.String())
	}
	back, _ := readTask(task.path)
	if !back.Due.Equal(task.Due) || !back.HasTime {
		t.Errorf("read back %v, want %v", back.Due, task.Due)
	}
}
