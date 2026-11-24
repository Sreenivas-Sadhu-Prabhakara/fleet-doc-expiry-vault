package fleet

import (
	"strings"
	"testing"
)

func TestRenewalChecklist(t *testing.T) {
	tests := []struct {
		kind      DocKind
		wantItems []string // substrings that must appear somewhere in the list
		wantForms []string
		wantWhere string
	}{
		{
			kind:      KindInsurance,
			wantItems: []string{"policy", "PUC", "no-claim bonus"},
			wantWhere: "insurer",
		},
		{
			kind:      KindFitness,
			wantItems: []string{"inspection", "insurance", "PUC", "Road tax"},
			wantForms: []string{"Form 25"},
			wantWhere: "testing station",
		},
		{
			kind:      KindPermit,
			wantItems: []string{"permit", "fitness", "Tax clearance"},
			wantWhere: "transport authority",
		},
		{
			kind:      KindPUC,
			wantItems: []string{"vehicle", "Registration number", "fee"},
			wantWhere: "PUC testing centre",
		},
		{
			kind:      KindRC,
			wantItems: []string{"registration certificate", "Chassis number"},
			wantForms: []string{"Form 25"},
			wantWhere: "RTO",
		},
		{
			kind:      KindRoadTax,
			wantItems: []string{"tax payment receipt", "insurance"},
			wantWhere: "state transport department",
		},
		{
			kind:      KindDrivingLicence,
			wantItems: []string{"driving licence", "address", "fee"},
			wantForms: []string{"Form 9", "Form 1A"},
			wantWhere: "RTO",
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			got, err := RenewalChecklist(tc.kind)
			if err != nil {
				t.Fatalf("RenewalChecklist(%s): %v", tc.kind, err)
			}
			if got.Kind != tc.kind {
				t.Errorf("Kind = %s, want %s", got.Kind, tc.kind)
			}
			if got.Label != tc.kind.Label() {
				t.Errorf("Label = %q, want %q", got.Label, tc.kind.Label())
			}
			if len(got.Items) < 3 {
				t.Errorf("Items has %d entries, want a usable list", len(got.Items))
			}
			if !strings.Contains(got.Where, tc.wantWhere) {
				t.Errorf("Where = %q, want it to mention %q", got.Where, tc.wantWhere)
			}
			joined := strings.Join(got.Items, " | ")
			for _, want := range tc.wantItems {
				if !strings.Contains(joined, want) {
					t.Errorf("the item list does not mention %q: %s", want, joined)
				}
			}
			joinedForms := strings.Join(got.Forms, " | ")
			for _, want := range tc.wantForms {
				if !strings.Contains(joinedForms, want) {
					t.Errorf("the form list does not mention %q: %s", want, joinedForms)
				}
			}
			// Every checklist must carry the honesty note, because none of
			// these lists is a substitute for asking the RTO.
			if !strings.Contains(got.Note, "Confirm the current requirement") {
				t.Errorf("Note = %q, want the confirm-with-the-RTO warning", got.Note)
			}
			if !strings.Contains(got.Note, "Indicative") {
				t.Errorf("Note = %q, want it labelled indicative", got.Note)
			}
		})
	}
}

func TestRenewalChecklistCoversEveryKind(t *testing.T) {
	for _, kind := range AllKinds() {
		if _, err := RenewalChecklist(kind); err != nil {
			t.Errorf("no checklist for %s: %v", kind, err)
		}
	}
	if got := len(AllChecklists()); got != len(AllKinds()) {
		t.Errorf("AllChecklists returned %d lists, want %d", got, len(AllKinds()))
	}
}

func TestRenewalChecklistRejectsUnknownKinds(t *testing.T) {
	for _, bad := range []DocKind{"", "Pizza", "insurance policy"} {
		if _, err := RenewalChecklist(bad); err == nil {
			t.Errorf("RenewalChecklist(%q) returned a list, want an error", bad)
		}
	}
}

// TestRenewalChecklistAcceptsShorthand lets a fleet owner ask for "cf" or "dl"
// and still get the right list.
func TestRenewalChecklistAcceptsShorthand(t *testing.T) {
	tests := map[string]DocKind{
		"cf":              KindFitness,
		"fc":              KindFitness,
		"FITNESS":         KindFitness,
		"dl":              KindDrivingLicence,
		"driving license": KindDrivingLicence,
		"pollution":       KindPUC,
		"tax":             KindRoadTax,
		"road_tax":        KindRoadTax,
		"registration":    KindRC,
	}
	for input, want := range tests {
		got, err := ParseDocKind(input)
		if err != nil {
			t.Errorf("ParseDocKind(%q): %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseDocKind(%q) = %s, want %s", input, got, want)
		}
		if _, err := RenewalChecklist(got); err != nil {
			t.Errorf("RenewalChecklist(%s): %v", got, err)
		}
	}
}
