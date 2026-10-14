package main

import (
	"strings"
	"testing"
)

func TestTree(t *testing.T) {
	task := func(uid, parent string) *Task { return &Task{UID: uid, Parent: parent, Summary: uid} }
	// Sorted by urgency, as pending() returns them. "compare" is the most
	// urgent, so its whole branch (trip > flights) comes first.
	ts := []*Task{
		task("compare", "flights"),
		task("rent", ""),
		task("flights", "trip"),
		task("pack", "trip"),
		task("trip", ""),
		task("orphan", "gone"),         // parent done or deleted
		task("a", "b"), task("b", "a"), // a cycle must not hang or vanish
	}
	var got []string
	for _, n := range tree(ts) {
		got = append(got, n.pre+n.Summary)
	}
	want := []string{
		"trip",
		"├─ flights",
		"│  └─ compare",
		"└─ pack",
		"rent",
		"orphan",
		"a",
		"└─ b",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tree:\n%s", strings.Join(got, "\n"))
	}
}
