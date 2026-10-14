package main

import "sort"

// A subtask points at its parent with RELATED-TO (RFC 5545). The tree is
// built from the urgency-sorted task list: siblings keep that order, and a
// parent ranks as high as its most urgent descendant.

type node struct {
	*Task
	pre  string // branches before the summary, e.g. "│  ├─ "
	cont string // prefix for lines under this task (notes, subtasks)
	kids bool
	top  *Task // most urgent task in this subtree
}

// tree orders open tasks (sorted by urgency) as a flattened tree.
func tree(ts []*Task) []node {
	rank := map[string]int{}
	for i, t := range ts {
		rank[t.UID] = i
	}
	kids := map[string][]*Task{}
	var roots []*Task
	for _, t := range ts {
		if _, open := rank[t.Parent]; open && t.Parent != t.UID {
			kids[t.Parent] = append(kids[t.Parent], t)
		} else { // no parent, or parent done/deleted
			roots = append(roots, t)
		}
	}

	best := map[string]int{}
	var bestOf func(t *Task) int
	bestOf = func(t *Task) int {
		if b, ok := best[t.UID]; ok {
			return b
		}
		best[t.UID] = rank[t.UID] // set first: guards against RELATED-TO cycles
		b := rank[t.UID]
		for _, k := range kids[t.UID] {
			b = min(b, bestOf(k))
		}
		best[t.UID] = b
		return b
	}

	var out []node
	seen := map[*Task]bool{}
	var walk func(sibs []*Task, prefix string, root bool)
	walk = func(sibs []*Task, prefix string, root bool) {
		sort.SliceStable(sibs, func(i, j int) bool { return bestOf(sibs[i]) < bestOf(sibs[j]) })
		for i, t := range sibs {
			if seen[t] {
				continue
			}
			seen[t] = true
			branch, next := "├─ ", "│  "
			if i == len(sibs)-1 {
				branch, next = "└─ ", "   "
			}
			if root {
				branch, next = "", ""
			}
			out = append(out, node{t, prefix + branch, prefix + next, len(kids[t.UID]) > 0, ts[bestOf(t)]})
			walk(kids[t.UID], prefix+next, false)
		}
	}
	walk(roots, "", true)
	for _, t := range ts { // tasks caught in a cycle have no root
		if !seen[t] {
			walk([]*Task{t}, "", true)
		}
	}
	return out
}

// subtree returns t and all its descendants, done or not.
func subtree(t *Task, all []*Task) []*Task {
	out, in := []*Task{t}, map[string]bool{t.UID: true}
	for i := 0; i < len(out); i++ {
		for _, c := range all {
			if c.Parent == out[i].UID && !in[c.UID] {
				out, in[c.UID] = append(out, c), true
			}
		}
	}
	return out
}
