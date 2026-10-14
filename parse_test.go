package main

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 13, 19, 0, 0, 0, time.Local) // a Tuesday

func TestParse(t *testing.T) {
	cases := []struct {
		in, summary, due string
		prio             Priority
		list             string
	}{
		{"call mom fri 18:00 !! #home", "call mom", "2026-10-16 18:00", 2, "home"},
		{"buy milk tomorrow", "buy milk", "2026-10-14 00:00", 0, ""},
		{"standup 09:30", "standup", "2026-10-14 09:30", 0, ""}, // already past today
		{"tax 20.10 !!!", "tax", "2026-10-20 00:00", 1, ""},
		{"ping +2h", "ping", "2026-10-13 21:00", 0, ""},
		{"water tue", "water", "2026-10-20 00:00", 0, ""}, // next Tuesday, not today
		{"read book", "read book", "0001-01-01 00:00", 0, ""},
		{"see https://go.dev tomorrow // and the blog", "see https://go.dev", "2026-10-14 00:00", 0, ""},
	}
	for _, c := range cases {
		p := parse(c.in, now)
		if got := p.Due.Format("2006-01-02 15:04"); p.Summary != c.summary || got != c.due || p.Prio != c.prio || p.List != c.list {
			t.Errorf("parse(%q) = %q %s %d %q", c.in, p.Summary, got, p.Prio, p.List)
		}
	}
}

func TestEditRoundTrip(t *testing.T) {
	orig := parse("finish report fri 09:30 !!! #work // send to Anna", now)
	got := parse(editText(&orig), now)
	if got.Summary != orig.Summary || !got.Due.Equal(orig.Due) || got.HasTime != orig.HasTime || got.Prio != orig.Prio || got.List != orig.List || got.Note != orig.Note {
		t.Errorf("round trip changed task: %+v -> %+v", orig, got)
	}
}

func TestLinkify(t *testing.T) {
	cases := map[string]string{
		"see https://go.dev.":             "see [[https://go.dev|LINK]].",
		"[[https://go.dev|Go]] and [[x]]": "[[https://go.dev|Go]] and [[x|LINK]]",
		"[[Daily note|today]]":            "[[Daily note|today]]",
	}
	for in, want := range cases {
		if got := linkify(in); got != want {
			t.Errorf("linkify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNoteLines(t *testing.T) {
	got := noteLines("buy milk at [[https://shop.de|the corner shop]] <now>\nsecond", 16)
	want := []string{
		"buy milk at <span color='#8ab4f8' underline='single'>the</span>", // 15 visible chars
		"<span color='#8ab4f8' underline='single'>corner</span> <span color='#8ab4f8' underline='single'>shop</span>",
		"&lt;now&gt;",
		"second",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("noteLines:\n%s", strings.Join(got, "\n"))
	}
}
