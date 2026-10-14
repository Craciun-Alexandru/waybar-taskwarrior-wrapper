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
call mom fri 18:00 !! #home // ask about Sunday
```

- **Due:** `today`, `tomorrow`, `fri`/`friday`, `+3d`, `+2w`, `+2h`, `20.10`, `20.10.2026`, `2026-10-20`, optionally with a `18:00` time. A time alone means today, or tomorrow if it has already passed.
- **Priority:** `!` low, `!!` medium, `!!!` high.
- **List:** `#name`. A new name creates the list. The default list is `personal`.
- **Note:** everything after ` //` (the space before `//` is required, so URLs are safe), e.g. `call mom fri // ask about Sunday`. This is the "Notes" field in iOS Reminders, shown in full under the task in the tooltip, wrapped to the tooltip's width.
  Links use Obsidian syntax, `[[https://example.com|name]]`, and the tooltip shows only `name`, underlined in blue. When a note is saved, bare URLs and `[[target]]` links are converted to `[[target|LINK]]`.

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

## Sync (Radicale + vdirsyncer, iPhone Reminders)

Example files are in [`sync/`](sync/). The server runs [Radicale](https://radicale.org) as a systemd user service on localhost,
and `tailscale serve` exposes it over HTTPS to the tailnet. iOS won't send a CalDAV password over plain HTTP, so HTTPS is required. The laptop runs vdirsyncer every 2 minutes, and wtodo also starts a sync after each change.

**Server**

```bash
python3 -m venv ~/.local/share/radicale/venv
~/.local/share/radicale/venv/bin/pip install radicale bcrypt
# users file: "<user>:<bcrypt hash>"; config and unit from sync/
systemctl --user enable --now radicale.service
# one-time: enable HTTPS certificates in the Tailscale admin console, then
sudo tailscale set --operator=$USER
tailscale serve --bg http://127.0.0.1:5232
```

Pre-create each list as task-only, so iOS shows it in Reminders rather than Calendar:
`collection-root/<user>/<list>/.Radicale.props` = `{"tag": "VCALENDAR", "D:displayname": "Work", "C:supported-calendar-component-set": "VTODO"}`.
Lists created later from either side are picked up automatically.

**Laptop**

```bash
pipx install vdirsyncer
# ~/.config/vdirsyncer/config from sync/, password in ~/.config/vdirsyncer/radicale-password
systemctl --user enable --now wtodo-sync.timer
```

**iPhone**: Settings → Apps → Reminders → Reminder Accounts → Add Account → Other → Add CalDAV Account,
with server `<host>.<tailnet>.ts.net`. Keep Tailscale on, with VPN On Demand.
