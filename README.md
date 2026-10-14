# wtodo

A small to-do module for waybar, with a fuzzel menu. The bar shows your most pressing task, and hovering shows all open tasks grouped by due date.

Tasks are plain iCalendar files in `~/.local/share/wtodo/<list>/*.ics`, the
vdir layout [vdirsyncer](https://vdirsyncer.pimutils.org) syncs, so they can be synced to a CalDAV server (and from
there to iOS Reminders) later.

## Install

```bash
go build -o ~/.local/bin/wtodo .
```

## Commands

| | |
|---|---|
| `wtodo waybar` | JSON for the waybar module |
| `wtodo menu` | fuzzel menu: pick a task to complete / snooze / edit / delete; type text that matches nothing to add it |
| `wtodo add [text]` | add a task (prompts with fuzzel if no text) |
| `wtodo done` | complete the task shown on the bar |
| `wtodo list` | print open tasks |

## Quick-add syntax

```
call mom fri 18:00 !! #home
```

- **Due:** `today`, `tomorrow`, `fri`/`friday`, `+3d`, `+2w`, `+2h`, `20.10`, `20.10.2026`, `2026-10-20`, optionally with a `18:00` time. A time alone means today, or tomorrow if it has already passed.
- **Priority:** `!` low, `!!` medium, `!!!` high.
- **List:** `#name`. A new name creates the list. The default list is `personal`.

**Edit…** in the menu pre-fills the task in this syntax, so one prompt edits everything.

## Ordering

Tasks due by the end of today (or overdue) come first, then by priority, then by due date. Undated tasks come last within their priority.

## Waybar

```jsonc
"custom/todo": {
  "exec": "~/.local/bin/wtodo waybar",
  "return-type": "json",
  "interval": 30,
  "signal": 8,
  "on-click": "pkill -x fuzzel || ~/.local/bin/wtodo menu",
  "on-click-right": "pkill -x fuzzel || ~/.local/bin/wtodo add",
  "on-click-middle": "~/.local/bin/wtodo done"
}
```

CSS classes: `overdue`, `today`, `tomorrow`, `later`, `nodate`, `empty`.
