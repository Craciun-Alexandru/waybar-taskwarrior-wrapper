package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

const linkColor = "#8ab4f8"

// Obsidian-style [[target|name]] links, or a bare URL.
var linkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]*))?\]\]|https?://[^\s\]]+`)

// linkify rewrites bare URLs and nameless [[target]] links to [[target|LINK]].
func linkify(note string) string {
	return linkRe.ReplaceAllStringFunc(note, func(m string) string {
		if sub := linkRe.FindStringSubmatch(m); sub[1] != "" {
			return "[[" + sub[1] + "|" + orDefault(sub[2], "LINK") + "]]"
		}
		url := strings.TrimRight(m, ".,;:!?)") // sentence punctuation stays outside
		return "[[" + url + "|LINK]]" + m[len(url):]
	})
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// noteLines renders a note as Pango lines wrapped at width visible characters.
// Links show only their name, underlined and blue.
func noteLines(note string, width int) []string {
	type word struct {
		text string
		link bool
	}
	var lines []string
	for _, para := range strings.Split(linkify(note), "\n") {
		var words []word
		last := 0
		for _, m := range linkRe.FindAllStringSubmatchIndex(para, -1) {
			for _, w := range strings.Fields(para[last:m[0]]) {
				words = append(words, word{w, false})
			}
			for _, w := range strings.Fields(para[m[4]:m[5]]) {
				words = append(words, word{w, true})
			}
			last = m[1]
		}
		for _, w := range strings.Fields(para[last:]) {
			words = append(words, word{w, false})
		}

		line, n := "", 0
		for _, w := range words {
			l := utf8.RuneCountInString(w.text)
			if n > 0 && n+1+l > width {
				lines, line, n = append(lines, line), "", 0
			}
			if n > 0 {
				line, n = line+" ", n+1
			}
			text := html.EscapeString(w.text)
			if w.link {
				text = fmt.Sprintf("<span color='%s' underline='single'>%s</span>", linkColor, text)
			}
			line, n = line+text, n+l
		}
		if n > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
