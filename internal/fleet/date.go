package fleet

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ISODate is the canonical date format used on the wire, in the data file and
// in CSV exports.
const ISODate = "2006-01-02"

// Date is a calendar date with no clock time and no zone.
//
// Document expiry is a calendar fact: a certificate of fitness that runs out on
// 31 March runs out on 31 March whatever the hour or the timezone of the laptop
// reading it. Carrying a time.Time here would invite "expired overnight" bugs
// from daylight saving and zone offsets, so the domain carries only Y/M/D and
// converts to midnight UTC — where every day is exactly 24 hours — whenever it
// has to do arithmetic.
type Date struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Day   int        `json:"day"`
}

// NewDate builds a date from its parts without validating them; use ParseDate
// for anything that came from a human.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// FromTime takes the calendar date of t in t's own location.
func FromTime(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Today is the current calendar date in the machine's local zone.
func Today() Date { return FromTime(time.Now()) }

// Time renders the date as midnight UTC so that subtraction gives whole days.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// IsZero reports whether no date has been recorded.
func (d Date) IsZero() bool { return d == Date{} }

// String renders the date as YYYY-MM-DD, or "" when unset.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format(ISODate)
}

// Pretty renders the date the way an Indian RTO document prints it, e.g.
// "31-03-2027". Empty when unset.
func (d Date) Pretty() string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("02-01-2006")
}

// MarshalJSON writes the date as a "YYYY-MM-DD" string (null when unset) so the
// data file and the API stay readable with a text editor.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON accepts "YYYY-MM-DD", "" and null.
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*d = Date{}
		return nil
	}
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("date: expected a quoted YYYY-MM-DD string, got %s", s)
	}
	inner := s[1 : len(s)-1]
	if strings.TrimSpace(inner) == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(inner)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// ErrEmptyDate is returned by ParseDate for blank input, so callers can tell
// "nothing entered" apart from "entered wrongly".
var ErrEmptyDate = errors.New("date: no date given")

// ParseDate reads a date typed by a human.
//
// YYYY-MM-DD is the canonical form. Day-first forms (DD-MM-YYYY, DD/MM/YYYY,
// DD.MM.YYYY) are also accepted because that is how dates are printed on Indian
// RC books, insurance policies and fitness certificates, and how they come out
// of a fleet owner's spreadsheet. A day-first string is always read day-first:
// 01-02-2027 is 1 February 2027, never 2 January.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, ErrEmptyDate
	}
	layouts := []string{ISODate, "2006/01/02", "02-01-2006", "02/01/2006", "02.01.2006"}
	for _, layout := range layouts {
		if len(layout) != len(s) {
			continue
		}
		if t, err := time.Parse(layout, s); err == nil {
			return FromTime(t), nil
		}
	}
	return Date{}, fmt.Errorf("date: %q is not a date — use YYYY-MM-DD or DD-MM-YYYY", s)
}

// Before reports whether d falls earlier in the calendar than other.
func (d Date) Before(other Date) bool { return d.Time().Before(other.Time()) }

// After reports whether d falls later in the calendar than other.
func (d Date) After(other Date) bool { return d.Time().After(other.Time()) }

// DaysBetween counts whole calendar days from one date to the next. It is
// negative when to falls before from, and counts leap days correctly because
// both ends are midnight UTC.
func DaysBetween(from, to Date) int {
	return int(to.Time().Sub(from.Time()) / (24 * time.Hour))
}

// DaysInMonth gives the length of a month, 29 for February in a leap year.
func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// AddMonths moves the date on by n calendar months, clamping to the end of the
// target month instead of spilling into the next one.
//
// This matters for real validity periods: a PUC certificate issued on 31 August
// and valid for six months runs to 29 February (28 February in a common year),
// not to 2 or 3 March as naive date addition would have it.
func (d Date) AddMonths(n int) Date {
	if d.IsZero() {
		return d
	}
	total := int(d.Month) - 1 + n
	year := d.Year + total/12
	month := total % 12
	if month < 0 {
		month += 12
		year--
	}
	m := time.Month(month + 1)
	day := d.Day
	if last := DaysInMonth(year, m); day > last {
		day = last
	}
	return Date{Year: year, Month: m, Day: day}
}

// AddDays moves the date on by n days.
func (d Date) AddDays(n int) Date {
	if d.IsZero() {
		return d
	}
	return FromTime(d.Time().AddDate(0, 0, n))
}
