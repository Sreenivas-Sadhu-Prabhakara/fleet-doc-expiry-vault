package fleet

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Date
		wantErr bool
	}{
		{name: "ISO", in: "2027-03-31", want: NewDate(2027, time.March, 31)},
		{name: "ISO with slashes", in: "2027/03/31", want: NewDate(2027, time.March, 31)},
		{name: "day first with hyphens, as printed on an RC", in: "31-03-2027", want: NewDate(2027, time.March, 31)},
		{name: "day first with slashes", in: "31/03/2027", want: NewDate(2027, time.March, 31)},
		{name: "day first with dots", in: "31.03.2027", want: NewDate(2027, time.March, 31)},
		{name: "ambiguous input is read day first", in: "01-02-2027", want: NewDate(2027, time.February, 1)},
		{name: "surrounding spaces are trimmed", in: "  2027-03-31  ", want: NewDate(2027, time.March, 31)},
		{name: "29 February in a leap year", in: "2028-02-29", want: NewDate(2028, time.February, 29)},
		{name: "empty", in: "", wantErr: true},
		{name: "only spaces", in: "   ", wantErr: true},
		{name: "29 February in a common year is rejected", in: "2027-02-29", wantErr: true},
		{name: "31 February is rejected", in: "2027-02-31", wantErr: true},
		{name: "month 13 is rejected", in: "2027-13-01", wantErr: true},
		{name: "not a date at all", in: "next month", wantErr: true},
		{name: "two digit year is rejected as too risky to guess", in: "31-03-27", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDate(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDate(%q) = %s, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseDate(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseDateEmptyIsItsOwnError(t *testing.T) {
	if _, err := ParseDate("  "); !errors.Is(err, ErrEmptyDate) {
		t.Errorf("ParseDate(\"  \") error = %v, want ErrEmptyDate so a blank field reads differently from a bad one", err)
	}
	if _, err := ParseDate("rubbish"); errors.Is(err, ErrEmptyDate) {
		t.Error("a malformed date must not report as an empty one")
	}
}

func TestDaysBetween(t *testing.T) {
	tests := []struct {
		name string
		from Date
		to   Date
		want int
	}{
		{"same day", NewDate(2027, time.March, 1), NewDate(2027, time.March, 1), 0},
		{"one day forward", NewDate(2027, time.March, 1), NewDate(2027, time.March, 2), 1},
		{"one day back", NewDate(2027, time.March, 2), NewDate(2027, time.March, 1), -1},
		{"across a 31-day month", NewDate(2027, time.January, 1), NewDate(2027, time.February, 1), 31},
		{"across a 28-day February", NewDate(2027, time.February, 1), NewDate(2027, time.March, 1), 28},
		{"across a 29-day February", NewDate(2028, time.February, 1), NewDate(2028, time.March, 1), 29},
		{"a common year", NewDate(2026, time.March, 1), NewDate(2027, time.March, 1), 365},
		{"a leap year", NewDate(2027, time.March, 1), NewDate(2028, time.March, 1), 366},
		{"four years spans one leap day", NewDate(2026, time.January, 1), NewDate(2030, time.January, 1), 1461},
		{"across the end of a century that is not a leap year", NewDate(2100, time.January, 1), NewDate(2101, time.January, 1), 365},
		// India keeps a single half-hour-offset zone and observes no daylight
		// saving, but the arithmetic must not depend on that either way.
		{"across the last Sunday in March, when many zones shift the clock", NewDate(2027, time.March, 27), NewDate(2027, time.March, 29), 2},
		{"across the last Sunday in October", NewDate(2027, time.October, 30), NewDate(2027, time.November, 1), 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DaysBetween(tc.from, tc.to); got != tc.want {
				t.Errorf("DaysBetween(%s, %s) = %d, want %d", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2027, time.January, 31},
		{2027, time.February, 28},
		{2028, time.February, 29},
		{2000, time.February, 29},
		{1900, time.February, 28},
		{2100, time.February, 28},
		{2027, time.April, 30},
		{2027, time.December, 31},
	}
	for _, tc := range tests {
		if got := DaysInMonth(tc.year, tc.month); got != tc.want {
			t.Errorf("DaysInMonth(%d, %s) = %d, want %d", tc.year, tc.month, got, tc.want)
		}
	}
}

func TestAddMonthsClampsToTheEndOfTheMonth(t *testing.T) {
	tests := []struct {
		name  string
		start Date
		add   int
		want  Date
	}{
		{"plain six months", NewDate(2027, time.January, 15), 6, NewDate(2027, time.July, 15)},
		{"a PUC issued on 31 August runs to 28 February, not into March", NewDate(2026, time.August, 31), 6, NewDate(2027, time.February, 28)},
		{"the same, into a leap February", NewDate(2027, time.August, 31), 6, NewDate(2028, time.February, 29)},
		{"31 January plus one month is 28 February", NewDate(2027, time.January, 31), 1, NewDate(2027, time.February, 28)},
		{"31 March plus one month is 30 April", NewDate(2027, time.March, 31), 1, NewDate(2027, time.April, 30)},
		{"29 February plus twelve months is 28 February", NewDate(2028, time.February, 29), 12, NewDate(2029, time.February, 28)},
		{"29 February plus forty-eight months is 29 February again", NewDate(2028, time.February, 29), 48, NewDate(2032, time.February, 29)},
		{"twelve months crosses the year", NewDate(2027, time.December, 1), 12, NewDate(2028, time.December, 1)},
		{"sixty months is five years", NewDate(2027, time.April, 10), 60, NewDate(2032, time.April, 10)},
		{"twenty-four months is two years", NewDate(2027, time.April, 10), 24, NewDate(2029, time.April, 10)},
		{"adding nothing changes nothing", NewDate(2027, time.April, 10), 0, NewDate(2027, time.April, 10)},
		{"going backwards works too", NewDate(2027, time.March, 31), -1, NewDate(2027, time.February, 28)},
		{"backwards across the year boundary", NewDate(2027, time.January, 15), -1, NewDate(2026, time.December, 15)},
		{"backwards a whole year", NewDate(2027, time.January, 15), -12, NewDate(2026, time.January, 15)},
		{"backwards thirteen months", NewDate(2027, time.January, 31), -13, NewDate(2025, time.December, 31)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.start.AddMonths(tc.add); got != tc.want {
				t.Errorf("%s.AddMonths(%d) = %s, want %s", tc.start, tc.add, got, tc.want)
			}
		})
	}
	if got := (Date{}).AddMonths(6); !got.IsZero() {
		t.Errorf("an unset date must stay unset, got %s", got)
	}
}

func TestAddDays(t *testing.T) {
	tests := []struct {
		start Date
		add   int
		want  Date
	}{
		{NewDate(2028, time.February, 28), 1, NewDate(2028, time.February, 29)},
		{NewDate(2028, time.February, 28), 2, NewDate(2028, time.March, 1)},
		{NewDate(2027, time.February, 28), 1, NewDate(2027, time.March, 1)},
		{NewDate(2027, time.March, 1), -1, NewDate(2027, time.February, 28)},
		{NewDate(2027, time.December, 31), 1, NewDate(2028, time.January, 1)},
		{NewDate(2027, time.January, 1), -1, NewDate(2026, time.December, 31)},
	}
	for _, tc := range tests {
		if got := tc.start.AddDays(tc.add); got != tc.want {
			t.Errorf("%s.AddDays(%d) = %s, want %s", tc.start, tc.add, got, tc.want)
		}
	}
}

func TestDateStringAndPretty(t *testing.T) {
	d := NewDate(2027, time.March, 9)
	if got := d.String(); got != "2027-03-09" {
		t.Errorf("String = %q, want 2027-03-09", got)
	}
	if got := d.Pretty(); got != "09-03-2027" {
		t.Errorf("Pretty = %q, want 09-03-2027", got)
	}
	if got := (Date{}).String(); got != "" {
		t.Errorf("an unset date renders as %q, want the empty string", got)
	}
	if got := (Date{}).Pretty(); got != "" {
		t.Errorf("an unset date renders prettily as %q, want the empty string", got)
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	type holder struct {
		On Date `json:"on"`
	}
	tests := []struct {
		name     string
		in       string
		want     Date
		wantJSON string
		wantErr  bool
	}{
		{name: "ISO string", in: `{"on":"2027-03-31"}`, want: NewDate(2027, time.March, 31), wantJSON: `{"on":"2027-03-31"}`},
		{name: "empty string", in: `{"on":""}`, want: Date{}, wantJSON: `{"on":""}`},
		{name: "null", in: `{"on":null}`, want: Date{}, wantJSON: `{"on":""}`},
		{name: "missing field", in: `{}`, want: Date{}, wantJSON: `{"on":""}`},
		{name: "day-first string from a hand-edited file", in: `{"on":"31-03-2027"}`, want: NewDate(2027, time.March, 31), wantJSON: `{"on":"2027-03-31"}`},
		{name: "a number is refused", in: `{"on":20270331}`, wantErr: true},
		{name: "nonsense is refused", in: `{"on":"soon"}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var h holder
			err := json.Unmarshal([]byte(tc.in), &h)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = %s, want an error", tc.in, h.On)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s): %v", tc.in, err)
			}
			if h.On != tc.want {
				t.Errorf("Unmarshal(%s) = %s, want %s", tc.in, h.On, tc.want)
			}
			out, err := json.Marshal(h)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(out) != tc.wantJSON {
				t.Errorf("Marshal = %s, want %s", out, tc.wantJSON)
			}
		})
	}
}

func TestDateBeforeAfter(t *testing.T) {
	early := NewDate(2027, time.February, 28)
	late := NewDate(2027, time.March, 1)
	if !early.Before(late) {
		t.Error("28 February falls before 1 March")
	}
	if !late.After(early) {
		t.Error("1 March falls after 28 February")
	}
	if early.Before(early) || early.After(early) {
		t.Error("a date is neither before nor after itself")
	}
}

func TestFromTimeIgnoresTheClock(t *testing.T) {
	// Two instants hours apart on the same local day must give the same date,
	// and the zone offset must not shunt it to the day before or after.
	kolkata := time.FixedZone("IST", 5*3600+1800)
	morning := time.Date(2027, time.March, 1, 0, 30, 0, 0, kolkata)
	night := time.Date(2027, time.March, 1, 23, 45, 0, 0, kolkata)
	want := NewDate(2027, time.March, 1)
	if got := FromTime(morning); got != want {
		t.Errorf("FromTime(just after midnight) = %s, want %s", got, want)
	}
	if got := FromTime(night); got != want {
		t.Errorf("FromTime(late evening) = %s, want %s", got, want)
	}
}
