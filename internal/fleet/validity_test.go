package fleet

import (
	"strings"
	"testing"
	"time"
)

// TestValidityConstants pins the indicative statutory periods the documentation
// quotes. If one of them is amended, this test is where it is changed, once.
func TestValidityConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"PUC, new vehicle", PUCNewVehicleValidityMonths, 12},
		{"PUC, thereafter", PUCValidityMonths, 6},
		{"insurance", InsuranceValidityMonths, 12},
		{"fitness, first certificate on a new transport vehicle", FitnessNewTransportValidityMonths, 24},
		{"fitness, on renewal", FitnessRenewalValidityMonths, 12},
		{"permit", PermitValidityMonths, 60},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %d months, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestDefaultValidityTable(t *testing.T) {
	table := DefaultValidity()
	tests := []struct {
		kind         DocKind
		wantFirst    int
		wantRenewal  int
		wantKnownNew bool
		wantKnownOld bool
	}{
		{KindPUC, 12, 6, true, true},
		{KindInsurance, 12, 12, true, true},
		{KindFitness, 24, 12, true, true},
		{KindPermit, 60, 60, true, true},
		{KindRC, 0, 0, false, false},
		{KindRoadTax, 0, 0, false, false},
		{KindDrivingLicence, 0, 0, false, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			period, ok := table[tc.kind]
			if !ok {
				t.Fatalf("%s is missing from the validity table", tc.kind)
			}
			if period.FirstIssueMonths != tc.wantFirst {
				t.Errorf("FirstIssueMonths = %d, want %d", period.FirstIssueMonths, tc.wantFirst)
			}
			if period.RenewalMonths != tc.wantRenewal {
				t.Errorf("RenewalMonths = %d, want %d", period.RenewalMonths, tc.wantRenewal)
			}
			if period.Known(true) != tc.wantKnownNew {
				t.Errorf("Known(firstIssue) = %v, want %v", period.Known(true), tc.wantKnownNew)
			}
			if period.Known(false) != tc.wantKnownOld {
				t.Errorf("Known(renewal) = %v, want %v", period.Known(false), tc.wantKnownOld)
			}
		})
	}
	if len(table) != len(AllKinds()) {
		t.Errorf("the validity table has %d entries, want one per document kind (%d)", len(table), len(AllKinds()))
	}
}

func TestSuggestExpiry(t *testing.T) {
	tests := []struct {
		name       string
		kind       DocKind
		issuedOn   Date
		firstIssue bool
		want       Date
		wantOK     bool
	}{
		{
			name: "PUC on a new vehicle runs twelve months",
			kind: KindPUC, issuedOn: NewDate(2027, time.April, 10), firstIssue: true,
			want: NewDate(2028, time.April, 10), wantOK: true,
		},
		{
			name: "PUC thereafter runs six months",
			kind: KindPUC, issuedOn: NewDate(2027, time.April, 10),
			want: NewDate(2027, time.October, 10), wantOK: true,
		},
		{
			name: "a PUC issued on the last day of a long month clamps, it does not spill",
			kind: KindPUC, issuedOn: NewDate(2026, time.August, 31),
			want: NewDate(2027, time.February, 28), wantOK: true,
		},
		{
			name: "the same PUC into a leap February",
			kind: KindPUC, issuedOn: NewDate(2027, time.August, 31),
			want: NewDate(2028, time.February, 29), wantOK: true,
		},
		{
			name: "insurance runs twelve months either way",
			kind: KindInsurance, issuedOn: NewDate(2027, time.March, 1),
			want: NewDate(2028, time.March, 1), wantOK: true,
		},
		{
			name: "insurance on a new vehicle also defaults to twelve months",
			kind: KindInsurance, issuedOn: NewDate(2027, time.March, 1), firstIssue: true,
			want: NewDate(2028, time.March, 1), wantOK: true,
		},
		{
			name: "the first fitness certificate on a new transport vehicle runs two years",
			kind: KindFitness, issuedOn: NewDate(2027, time.June, 15), firstIssue: true,
			want: NewDate(2029, time.June, 15), wantOK: true,
		},
		{
			name: "every fitness renewal after that runs one year",
			kind: KindFitness, issuedOn: NewDate(2029, time.June, 15),
			want: NewDate(2030, time.June, 15), wantOK: true,
		},
		{
			name: "a fitness certificate issued on 29 February renews to 28 February",
			kind: KindFitness, issuedOn: NewDate(2028, time.February, 29),
			want: NewDate(2029, time.February, 28), wantOK: true,
		},
		{
			name: "a permit runs five years",
			kind: KindPermit, issuedOn: NewDate(2027, time.January, 1),
			want: NewDate(2032, time.January, 1), wantOK: true,
		},
		{
			name: "a permit issued on 29 February runs to 29 February five years on",
			kind: KindPermit, issuedOn: NewDate(2028, time.February, 29),
			want: NewDate(2033, time.February, 28), wantOK: true,
		},
		{
			name: "registration has no defensible default",
			kind: KindRC, issuedOn: NewDate(2027, time.January, 1),
		},
		{
			name: "road tax has no defensible default",
			kind: KindRoadTax, issuedOn: NewDate(2027, time.January, 1),
		},
		{
			name: "a driving licence has no defensible default",
			kind: KindDrivingLicence, issuedOn: NewDate(2027, time.January, 1),
		},
		{
			name: "no issue date, no suggestion",
			kind: KindInsurance,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SuggestExpiry(tc.kind, tc.issuedOn, tc.firstIssue, nil)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %s)", ok, tc.wantOK, got)
			}
			if ok && got != tc.want {
				t.Errorf("SuggestExpiry = %s, want %s", got, tc.want)
			}
		})
	}
}

// 2033 is not a leap year, so five years from 29 February 2028 clamps to the
// 28th. This spells that out, because it is exactly the sort of date a fleet
// owner would otherwise get wrong by hand.
func TestSuggestExpiryPermitFromALeapDay(t *testing.T) {
	got, ok := SuggestExpiry(KindPermit, NewDate(2028, time.February, 29), false, nil)
	if !ok {
		t.Fatal("a permit has a default validity period")
	}
	if got != NewDate(2033, time.February, 28) {
		t.Errorf("SuggestExpiry = %s, want 2033-02-28", got)
	}
	if DaysInMonth(2033, time.February) != 28 {
		t.Fatal("2033 is not a leap year, so there is no 29 February to clamp to")
	}
}

func TestSuggestExpiryHonoursOverrides(t *testing.T) {
	overrides := DefaultValidity()
	// A state that has moved to a three-month PUC cycle, and a vehicle class
	// whose permit runs ten years.
	overrides[KindPUC] = ValidityPeriod{FirstIssueMonths: 12, RenewalMonths: 3}
	overrides[KindPermit] = ValidityPeriod{FirstIssueMonths: 120, RenewalMonths: 120}
	// And an authority that has started dating road tax receipts.
	overrides[KindRoadTax] = ValidityPeriod{FirstIssueMonths: 12, RenewalMonths: 12}

	tests := []struct {
		kind     DocKind
		issuedOn Date
		want     Date
	}{
		{KindPUC, NewDate(2027, time.April, 10), NewDate(2027, time.July, 10)},
		{KindPermit, NewDate(2027, time.April, 10), NewDate(2037, time.April, 10)},
		{KindRoadTax, NewDate(2027, time.April, 10), NewDate(2028, time.April, 10)},
	}
	for _, tc := range tests {
		got, ok := SuggestExpiry(tc.kind, tc.issuedOn, false, overrides)
		if !ok {
			t.Fatalf("%s: the override gives it a validity period", tc.kind)
		}
		if got != tc.want {
			t.Errorf("%s: SuggestExpiry = %s, want %s", tc.kind, got, tc.want)
		}
	}

	// Switching an override back off must stop the suggestion again.
	overrides[KindInsurance] = ValidityPeriod{}
	if _, ok := SuggestExpiry(KindInsurance, NewDate(2027, time.April, 10), false, overrides); ok {
		t.Error("a zeroed override must stop the vault suggesting a date")
	}
}

func TestFleetSetValidity(t *testing.T) {
	f := New()
	if err := f.SetValidity(KindPUC, ValidityPeriod{FirstIssueMonths: 12, RenewalMonths: 3}); err != nil {
		t.Fatalf("SetValidity: %v", err)
	}
	got, ok := f.SuggestExpiry(KindPUC, NewDate(2027, time.January, 31), false)
	if !ok {
		t.Fatal("PUC has a validity period")
	}
	if got != NewDate(2027, time.April, 30) {
		t.Errorf("SuggestExpiry = %s, want 2027-04-30", got)
	}
	if err := f.SetValidity(KindPermit, ValidityPeriod{RenewalMonths: -1}); err == nil {
		t.Error("a negative validity period must be refused")
	}
	if err := f.SetValidity("Pizza", ValidityPeriod{RenewalMonths: 6}); err == nil {
		t.Error("an unknown document kind must be refused")
	}
}

func TestValidityNoteExistsForEveryKind(t *testing.T) {
	for _, kind := range AllKinds() {
		if ValidityNote(kind) == "" {
			t.Errorf("%s has no validity note, so the docs would quote a figure with nothing behind it", kind)
		}
	}
	// Each kind with a default must say so, and each kind without one must say
	// there is none.
	for _, kind := range AllKinds() {
		note := ValidityNote(kind)
		hasDefault := DefaultValidity()[kind].Known(false) || DefaultValidity()[kind].Known(true)
		if hasDefault && !strings.Contains(note, "Indicative") {
			t.Errorf("%s has a default but its note does not call it indicative: %q", kind, note)
		}
		if !hasDefault && !strings.Contains(note, "No default") {
			t.Errorf("%s has no default but its note does not say so: %q", kind, note)
		}
	}
}
