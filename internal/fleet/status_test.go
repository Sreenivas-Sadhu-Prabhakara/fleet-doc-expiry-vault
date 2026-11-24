package fleet

import (
	"testing"
	"time"
)

func TestStatusTierBoundaries(t *testing.T) {
	asOf := NewDate(2027, time.March, 1)

	tests := []struct {
		name        string
		expiresOn   Date
		wantKind    StatusKind
		wantLeft    int
		wantOverdue int
		wantTier    int
		wantBand    string
	}{
		{
			name:      "far future is valid",
			expiresOn: NewDate(2028, time.March, 1),
			wantKind:  StatusValid, wantLeft: 366, wantBand: BandGreen,
		},
		{
			name:      "day 31 is still valid, one day outside the first tier",
			expiresOn: NewDate(2027, time.April, 1),
			wantKind:  StatusValid, wantLeft: 31, wantBand: BandGreen,
		},
		{
			name:      "day 30 fires the 30-day tier",
			expiresOn: NewDate(2027, time.March, 31),
			wantKind:  StatusDueSoon, wantLeft: 30, wantTier: TierNoticeDays, wantBand: BandAmber,
		},
		{
			name:      "day 16 is inside the 30-day tier, not yet the 15-day tier",
			expiresOn: NewDate(2027, time.March, 17),
			wantKind:  StatusDueSoon, wantLeft: 16, wantTier: TierNoticeDays, wantBand: BandAmber,
		},
		{
			name:      "day 15 fires the 15-day tier",
			expiresOn: NewDate(2027, time.March, 16),
			wantKind:  StatusDueSoon, wantLeft: 15, wantTier: TierWarnDays, wantBand: BandAmber,
		},
		{
			name:      "day 4 is inside the 15-day tier, not yet the 3-day tier",
			expiresOn: NewDate(2027, time.March, 5),
			wantKind:  StatusDueSoon, wantLeft: 4, wantTier: TierWarnDays, wantBand: BandAmber,
		},
		{
			name:      "day 3 fires the 3-day tier",
			expiresOn: NewDate(2027, time.March, 4),
			wantKind:  StatusDueSoon, wantLeft: 3, wantTier: TierUrgentDays, wantBand: BandAmber,
		},
		{
			name:      "day 1 is the last full day",
			expiresOn: NewDate(2027, time.March, 2),
			wantKind:  StatusDueSoon, wantLeft: 1, wantTier: TierUrgentDays, wantBand: BandAmber,
		},
		{
			name:      "day 0, the expiry day itself, is still legal and maximally urgent",
			expiresOn: NewDate(2027, time.March, 1),
			wantKind:  StatusDueSoon, wantLeft: 0, wantTier: TierUrgentDays, wantBand: BandAmber,
		},
		{
			name:      "day minus 1, the morning after, is expired by one day",
			expiresOn: NewDate(2027, time.February, 28),
			wantKind:  StatusExpired, wantOverdue: 1, wantBand: BandRed,
		},
		{
			name:      "long overdue",
			expiresOn: NewDate(2026, time.March, 1),
			wantKind:  StatusExpired, wantOverdue: 365, wantBand: BandRed,
		},
		{
			name:      "no expiry date recorded is unknown, not valid",
			expiresOn: Date{},
			wantKind:  StatusUnknown, wantBand: BandGrey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Status(tc.expiresOn, asOf)
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.DaysLeft != tc.wantLeft {
				t.Errorf("DaysLeft = %d, want %d", got.DaysLeft, tc.wantLeft)
			}
			if got.DaysOverdue != tc.wantOverdue {
				t.Errorf("DaysOverdue = %d, want %d", got.DaysOverdue, tc.wantOverdue)
			}
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %d, want %d", got.Tier, tc.wantTier)
			}
			if got.Band != tc.wantBand {
				t.Errorf("Band = %q, want %q", got.Band, tc.wantBand)
			}
		})
	}
}

// TestStatusAcrossLeapDay checks that the 29th of February is counted as a real
// day. A fitness certificate running out at the start of March must not look a
// day further away in a leap year than it is.
func TestStatusAcrossLeapDay(t *testing.T) {
	tests := []struct {
		name      string
		asOf      Date
		expiresOn Date
		wantKind  StatusKind
		wantLeft  int
		wantTier  int
	}{
		{
			name: "leap year: 28 Feb to 1 Mar is two days because 29 Feb exists",
			asOf: NewDate(2028, time.February, 28), expiresOn: NewDate(2028, time.March, 1),
			wantKind: StatusDueSoon, wantLeft: 2, wantTier: TierUrgentDays,
		},
		{
			name: "common year: 28 Feb to 1 Mar is one day",
			asOf: NewDate(2027, time.February, 28), expiresOn: NewDate(2027, time.March, 1),
			wantKind: StatusDueSoon, wantLeft: 1, wantTier: TierUrgentDays,
		},
		{
			name: "29 February is a valid expiry date and lands exactly on the 30-day tier",
			asOf: NewDate(2028, time.January, 30), expiresOn: NewDate(2028, time.February, 29),
			wantKind: StatusDueSoon, wantLeft: 30, wantTier: TierNoticeDays,
		},
		{
			name: "the day before that is 31 days out and still valid",
			asOf: NewDate(2028, time.January, 29), expiresOn: NewDate(2028, time.February, 29),
			wantKind: StatusValid, wantLeft: 31,
		},
		{
			name: "standing on 29 February looking at 15 March",
			asOf: NewDate(2028, time.February, 29), expiresOn: NewDate(2028, time.March, 15),
			wantKind: StatusDueSoon, wantLeft: 15, wantTier: TierWarnDays,
		},
		{
			name: "a licence that ran out on 29 February is one day overdue on 1 March",
			asOf: NewDate(2028, time.March, 1), expiresOn: NewDate(2028, time.February, 29),
			wantKind: StatusExpired, wantLeft: 0,
		},
		{
			name: "a whole leap year is 366 days",
			asOf: NewDate(2028, time.January, 1), expiresOn: NewDate(2029, time.January, 1),
			wantKind: StatusValid, wantLeft: 366,
		},
		{
			name: "a whole common year is 365 days",
			asOf: NewDate(2029, time.January, 1), expiresOn: NewDate(2030, time.January, 1),
			wantKind: StatusValid, wantLeft: 365,
		},
		{
			name: "2100 is not a leap year: 28 Feb to 1 Mar is one day",
			asOf: NewDate(2100, time.February, 28), expiresOn: NewDate(2100, time.March, 1),
			wantKind: StatusDueSoon, wantLeft: 1, wantTier: TierUrgentDays,
		},
		{
			name: "2000 was a leap year: 28 Feb to 1 Mar is two days",
			asOf: NewDate(2000, time.February, 28), expiresOn: NewDate(2000, time.March, 1),
			wantKind: StatusDueSoon, wantLeft: 2, wantTier: TierUrgentDays,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Status(tc.expiresOn, tc.asOf)
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.DaysLeft != tc.wantLeft {
				t.Errorf("DaysLeft = %d, want %d", got.DaysLeft, tc.wantLeft)
			}
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %d, want %d", got.Tier, tc.wantTier)
			}
		})
	}
}

// TestStatusTiersAreTheNamedConstants guards the tier values themselves, since
// the whole reminder schedule is stated in the documentation.
func TestStatusTiersAreTheNamedConstants(t *testing.T) {
	if TierNoticeDays != 30 || TierWarnDays != 15 || TierUrgentDays != 3 {
		t.Fatalf("reminder tiers are %d/%d/%d, want 30/15/3",
			TierNoticeDays, TierWarnDays, TierUrgentDays)
	}
	if TierNoticeDays <= TierWarnDays || TierWarnDays <= TierUrgentDays {
		t.Fatalf("tiers must be strictly descending, got %d/%d/%d",
			TierNoticeDays, TierWarnDays, TierUrgentDays)
	}
}

// TestStatusEveryDayInTheFirstTier walks every single day of the reminder
// window, which is the cheapest way to prove no day falls through a gap or
// lands in two tiers at once.
func TestStatusEveryDayInTheFirstTier(t *testing.T) {
	asOf := NewDate(2027, time.June, 15)
	for left := -2; left <= 40; left++ {
		expiry := asOf.AddDays(left)
		got := Status(expiry, asOf)
		switch {
		case left < 0:
			if got.Kind != StatusExpired || got.DaysOverdue != -left {
				t.Errorf("%d days out: got %+v, want expired by %d", left, got, -left)
			}
		case left <= TierUrgentDays:
			if got.Kind != StatusDueSoon || got.Tier != TierUrgentDays || got.DaysLeft != left {
				t.Errorf("%d days out: got %+v, want due soon tier %d", left, got, TierUrgentDays)
			}
		case left <= TierWarnDays:
			if got.Kind != StatusDueSoon || got.Tier != TierWarnDays {
				t.Errorf("%d days out: got %+v, want due soon tier %d", left, got, TierWarnDays)
			}
		case left <= TierNoticeDays:
			if got.Kind != StatusDueSoon || got.Tier != TierNoticeDays {
				t.Errorf("%d days out: got %+v, want due soon tier %d", left, got, TierNoticeDays)
			}
		default:
			if got.Kind != StatusValid || got.DaysLeft != left {
				t.Errorf("%d days out: got %+v, want valid with %d days left", left, got, left)
			}
		}
	}
}

func TestStatusDescribe(t *testing.T) {
	asOf := NewDate(2027, time.March, 1)
	tests := []struct {
		expiresOn Date
		want      string
	}{
		{NewDate(2027, time.February, 28), "Expired 1 day ago"},
		{NewDate(2027, time.February, 1), "Expired 28 days ago"},
		{NewDate(2027, time.March, 1), "Expires today"},
		{NewDate(2027, time.March, 2), "1 day left"},
		{NewDate(2027, time.March, 20), "19 days left"},
		{Date{}, "No expiry date on file"},
	}
	for _, tc := range tests {
		if got := Status(tc.expiresOn, asOf).Describe(); got != tc.want {
			t.Errorf("Status(%s).Describe() = %q, want %q", tc.expiresOn, got, tc.want)
		}
	}
}
