package ui

import (
	"fmt"
	"strconv"
	"strings"
)

// CronEntry is one schedulable task, broken into the fields a form can
// edit directly. Raw holds the original line for entries the structured
// editor doesn't understand (so we never silently mangle something we
// can't fully parse); when Raw is set, the other schedule fields are
// ignored on save and the line is written back verbatim.
type CronEntry struct {
	ID       string `json:"id"`      // stable per-session index, not persisted
	Comment  string `json:"comment"` // text of a "# ..." line directly above this entry, if any
	Minute   string `json:"minute"`
	Hour     string `json:"hour"`
	DOM      string `json:"dom"` // day of month
	Month    string `json:"month"`
	DOW      string `json:"dow"` // day of week
	Command  string `json:"command"`
	IsReboot bool   `json:"is_reboot"` // true for @reboot lines (no time fields)
	Raw      string `json:"raw"`       // set only when the line couldn't be parsed into fields above
}

// ParseCrontab splits raw crontab text into entries, in file order.
// Blank lines are dropped. A comment line immediately preceding a task
// line is attached to that task as its description; standalone comment
// blocks (env var lines like PATH=, or comments not attached to a task)
// are kept as their own Raw entries so they're never lost.
func ParseCrontab(text string) []CronEntry {
	lines := strings.Split(text, "\n")
	var entries []CronEntry
	var pendingComment string

	flushPendingComment := func() {
		if pendingComment != "" {
			entries = append(entries, CronEntry{Raw: "# " + pendingComment})
			pendingComment = ""
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			flushPendingComment()
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			// Treat as a pending description for the *next* task line.
			// If another comment or non-task line follows instead, it
			// gets flushed as its own standalone raw entry untouched.
			flushPendingComment()
			pendingComment = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			continue
		}

		if strings.HasPrefix(trimmed, "@reboot") {
			cmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "@reboot"))
			entries = append(entries, CronEntry{
				Comment:  pendingComment,
				IsReboot: true,
				Command:  cmd,
			})
			pendingComment = ""
			continue
		}

		fields := strings.Fields(trimmed)

		// Env/assignment lines (e.g. PATH=/usr/bin) or other @-schedules
		// (@daily, @weekly, etc.) and anything else non-standard: keep
		// verbatim rather than guess.
		if strings.HasPrefix(trimmed, "@") || len(fields) < 6 || !looksLikeFiveFieldSchedule(fields) {
			flushPendingComment()
			entries = append(entries, CronEntry{Raw: line})
			continue
		}

		entries = append(entries, CronEntry{
			Comment: pendingComment,
			Minute:  fields[0],
			Hour:    fields[1],
			DOM:     fields[2],
			Month:   fields[3],
			DOW:     fields[4],
			Command: strings.Join(fields[5:], " "),
		})
		pendingComment = ""
	}
	flushPendingComment()

	for i := range entries {
		entries[i].ID = strconv.Itoa(i)
	}
	return entries
}

func looksLikeFiveFieldSchedule(fields []string) bool {
	if len(fields) < 6 {
		return false
	}
	for _, f := range fields[:5] {
		if strings.Contains(f, "=") {
			return false
		}
	}
	return true
}

// SerializeCrontab turns structured entries back into crontab text.
func SerializeCrontab(entries []CronEntry) string {
	var b strings.Builder
	for _, e := range entries {
		if e.Raw != "" {
			b.WriteString(e.Raw)
			b.WriteString("\n")
			continue
		}
		if e.Comment != "" {
			b.WriteString("# ")
			b.WriteString(e.Comment)
			b.WriteString("\n")
		}
		if e.IsReboot {
			b.WriteString("@reboot ")
			b.WriteString(e.Command)
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "%s %s %s %s %s %s\n",
			orStar(e.Minute), orStar(e.Hour), orStar(e.DOM), orStar(e.Month), orStar(e.DOW), e.Command)
	}
	return b.String()
}

func orStar(s string) string {
	if strings.TrimSpace(s) == "" {
		return "*"
	}
	return s
}

// DescribeSchedule returns a short, plain-language summary of a cron
// schedule for the live preview in the entry dialog. It deliberately
// keeps to common cases; anything irregular still shows the raw fields
// rather than a wrong-sounding guess.
func DescribeSchedule(e CronEntry) string {
	if e.IsReboot {
		return "Runs once when the server starts."
	}
	min, hour, dom, month, dow := e.Minute, e.Hour, e.DOM, e.Month, e.DOW

	allStar := dom == "*" && month == "*" && dow == "*"
	var timePart string
	switch {
	case isNumeric(hour) && isNumeric(min):
		timePart = fmt.Sprintf("at %02s:%02s", hour, min)
	case min == "*" && hour == "*":
		timePart = "every minute"
	default:
		timePart = fmt.Sprintf("at minute %s of hour %s", min, hour)
	}

	switch {
	case allStar:
		return "Runs every day " + timePart + "."
	case dom == "*" && month == "*" && dow != "*":
		return "Runs on " + weekdayNames(dow) + " " + timePart + "."
	case dom != "*" && month == "*" && dow == "*":
		return "Runs on day " + dom + " of every month " + timePart + "."
	default:
		return fmt.Sprintf("Runs %s when minute=%s hour=%s day-of-month=%s month=%s day-of-week=%s.",
			timePart, min, hour, dom, month, dow)
	}
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func weekdayNames(dow string) string {
	names := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday", "7": "Sunday",
	}
	parts := strings.Split(dow, ",")
	var out []string
	for _, p := range parts {
		if n, ok := names[strings.TrimSpace(p)]; ok {
			out = append(out, n)
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}
