package main

import (
	"strconv"
	"strings"
	"time"
)

const syntaxHelp = "tomorrow · fri · +3d · +2h · 20.10 · 2026-10-20 · 18:00 · !/!!/!!! priority · #list"

// parse turns "call mom fri 18:00 !! #home" into a task's fields.
// Recognised tokens are removed; everything else becomes the summary.
func parse(input string, now time.Time) Task {
	var t Task
	var words []string
	var date time.Time
	clock := -1 // minutes since midnight

	for _, w := range strings.Fields(input) {
		lw := strings.ToLower(w)
		if lw == "!" || lw == "!!" || lw == "!!!" {
			t.Prio = Priority(4 - len(lw)) // ! low, !! medium, !!! high
		} else if strings.HasPrefix(w, "#") && len(w) > 1 {
			t.List = strings.ToLower(w[1:])
		} else if m, ok := parseClock(lw); ok && clock < 0 {
			clock = m
		} else if d, timed, ok := parseDate(lw, now); ok && date.IsZero() {
			date = d
			if timed {
				clock = d.Hour()*60 + d.Minute()
				date = startOfDay(d)
			}
		} else {
			words = append(words, w)
		}
	}
	t.Summary = strings.Join(words, " ")

	switch {
	case !date.IsZero() && clock >= 0:
		t.Due, t.HasTime = date.Add(time.Duration(clock)*time.Minute), true
	case !date.IsZero():
		t.Due = date
	case clock >= 0: // time only: today, or tomorrow if already past
		t.Due, t.HasTime = startOfDay(now).Add(time.Duration(clock)*time.Minute), true
		if t.Due.Before(now) {
			t.Due = t.Due.AddDate(0, 0, 1)
		}
	}
	return t
}

// parseClock accepts "18:00" or "9:30".
func parseClock(s string) (int, bool) {
	h, m, ok := strings.Cut(s, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || len(m) != 2 || hh > 23 || mm > 59 {
		return 0, false
	}
	return hh*60 + mm, true
}

// parseDate returns a date (or an exact time for "+2h") and whether it's timed.
func parseDate(s string, now time.Time) (time.Time, bool, bool) {
	today := startOfDay(now)
	switch s {
	case "today":
		return today, false, true
	case "tomorrow", "tmr":
		return today.AddDate(0, 0, 1), false, true
	}
	for wd := time.Sunday; wd <= time.Saturday; wd++ { // "fri" or "friday": next one, 1-7 days ahead
		if name := strings.ToLower(wd.String()); s == name || s == name[:3] {
			return today.AddDate(0, 0, (int(wd)-int(now.Weekday())+6)%7+1), false, true
		}
	}
	if len(s) > 2 && s[0] == '+' { // +3d, +2w, +4h
		n, err := strconv.Atoi(s[1 : len(s)-1])
		if err != nil {
			return time.Time{}, false, false
		}
		switch s[len(s)-1] {
		case 'h':
			return now.Add(time.Duration(n) * time.Hour).Truncate(time.Minute), true, true
		case 'd':
			return today.AddDate(0, 0, n), false, true
		case 'w':
			return today.AddDate(0, 0, 7*n), false, true
		}
		return time.Time{}, false, false
	}
	if d, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return d, false, true
	}
	for _, layout := range []string{"2.1.2006", "2.1"} { // 20.10.2026, 20.10
		if d, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			if layout == "2.1" {
				d = d.AddDate(now.Year(), 0, 0)
				if d.Before(today) {
					d = d.AddDate(1, 0, 0)
				}
			}
			return d, false, true
		}
	}
	return time.Time{}, false, false
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
