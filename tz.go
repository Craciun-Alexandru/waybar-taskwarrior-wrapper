package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Due times are written in the local zone with a VTIMEZONE block, like iOS
// does. Written in UTC, iOS shows them as "20:58 (01:58 +1 Day GMT)".

// localZone returns the system zone (e.g. Europe/Berlin), or nil for UTC or
// if it has no IANA name.
func localZone() *time.Location {
	name := strings.TrimPrefix(os.Getenv("TZ"), ":")
	if name == "" {
		link, _ := os.Readlink("/etc/localtime")
		_, name, _ = strings.Cut(link, "zoneinfo/")
	}
	loc, err := time.LoadLocation(name)
	if err != nil || name == "" || name == "UTC" {
		return nil
	}
	return loc
}

// addTimezone adds a VTIMEZONE for loc to cal unless it already has one.
func addTimezone(cal *ical.Calendar, loc *time.Location, year int) {
	for _, c := range cal.Children {
		if id, _ := c.Props.Text(ical.PropTimezoneID); c.Name == ical.CompTimezone && id == loc.String() {
			return
		}
	}
	tz := ical.NewComponent(ical.CompTimezone)
	tz.Props.SetText(ical.PropTimezoneID, loc.String())

	// Follow the year's offset changes: two for DST zones, none otherwise.
	t := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	for i := 0; i < 2; i++ {
		_, next := t.ZoneBounds()
		if next.IsZero() || next.Year() != year {
			break
		}
		t = next
		_, from := t.Add(-time.Second).Zone()
		tz.Children = append(tz.Children, tzRule(t, from))
	}
	if len(tz.Children) == 0 { // no DST: one fixed offset
		_, off := t.Zone()
		tz.Children = append(tz.Children, tzRule(time.Date(1970, 1, 1, 0, 0, 0, 0, loc), off))
	}
	cal.Children = append([]*ical.Component{tz}, cal.Children...)
}

// tzRule describes the switch to the zone in effect at t, coming from offset
// from, as a STANDARD or DAYLIGHT rule repeating yearly on the same weekday
// of the month (e.g. second Sunday of March, last Sunday of October).
func tzRule(t time.Time, from int) *ical.Component {
	name, to := t.Zone()
	rule := ical.NewComponent(ical.CompTimezoneStandard)
	if t.IsDST() {
		rule.Name = ical.CompTimezoneDaylight
	}
	raw := func(k, v string) {
		p := ical.NewProp(k)
		p.Value = v
		rule.Props.Set(p)
	}
	raw("DTSTART", t.In(time.FixedZone("", from)).Format("20060102T150405")) // wall time before the switch
	raw("TZOFFSETFROM", utcOffset(from))
	raw("TZOFFSETTO", utcOffset(to))
	rule.Props.SetText("TZNAME", name)
	if t.Year() > 1970 {
		nth := (t.Day()-1)/7 + 1
		if t.AddDate(0, 0, 7).Month() != t.Month() {
			nth = -1 // last one in the month
		}
		raw("RRULE", fmt.Sprintf("FREQ=YEARLY;BYMONTH=%d;BYDAY=%d%s", t.Month(), nth, strings.ToUpper(t.Weekday().String()[:2])))
	}
	return rule
}

func utcOffset(sec int) string {
	sign := "+"
	if sec < 0 {
		sign, sec = "-", -sec
	}
	return fmt.Sprintf("%s%02d%02d", sign, sec/3600, sec%3600/60)
}
