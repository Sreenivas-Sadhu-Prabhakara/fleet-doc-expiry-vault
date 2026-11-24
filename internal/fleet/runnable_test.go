package fleet

import (
	"sort"
	"strings"
	"testing"
	"time"
)

// asOf for every CanRunToday case below.
var runDay = NewDate(2027, time.March, 1)

func lorry() Vehicle {
	return Vehicle{RegNo: "KA01AB1234", Type: TypeGoods, Make: "Tata 407", OwnerName: "Ravi Transport"}
}

// fullSet returns a complete, comfortably valid document set for the lorry, so
// each test can expire exactly the documents it is about.
func fullSet() []Document {
	return []Document{
		{VehicleRegNo: "KA01AB1234", Kind: KindRC, Number: "RC-1", ExpiresOn: NewDate(2030, time.January, 1)},
		{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, Number: "POL-9", ExpiresOn: NewDate(2028, time.January, 1)},
		{VehicleRegNo: "KA01AB1234", Kind: KindFitness, Number: "CF-3", ExpiresOn: NewDate(2028, time.February, 1)},
		{VehicleRegNo: "KA01AB1234", Kind: KindPermit, Number: "PRM-7", ExpiresOn: NewDate(2029, time.June, 1)},
		{VehicleRegNo: "KA01AB1234", Kind: KindPUC, Number: "PUC-2", ExpiresOn: NewDate(2027, time.August, 1)},
		{VehicleRegNo: "KA01AB1234", Kind: KindRoadTax, Number: "TAX-5", ExpiresOn: NewDate(2027, time.December, 31)},
	}
}

// expire moves the named kinds to a date in the past, the given number of days
// before the day being asked about.
func expire(docs []Document, daysOverdue int, kinds ...DocKind) []Document {
	out := make([]Document, len(docs))
	copy(out, docs)
	for _, kind := range kinds {
		for i := range out {
			if out[i].Kind == kind {
				out[i].ExpiresOn = runDay.AddDays(-daysOverdue)
			}
		}
	}
	return out
}

func drop(docs []Document, kinds ...DocKind) []Document {
	var out []Document
	for _, doc := range docs {
		keep := true
		for _, kind := range kinds {
			if doc.Kind == kind {
				keep = false
			}
		}
		if keep {
			out = append(out, doc)
		}
	}
	return out
}

func labels(items []Blocker) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item.Kind))
	}
	sort.Strings(out)
	return out
}

func TestCanRunToday(t *testing.T) {
	tests := []struct {
		name         string
		docs         []Document
		blocking     map[DocKind]bool
		wantCanRun   bool
		wantBlocking []string
		wantMissing  []string
		wantReported []string
	}{
		{
			name:       "every document valid: the vehicle can run",
			docs:       fullSet(),
			wantCanRun: true,
		},
		{
			name:         "insurance expired: blocked, naming insurance and nothing else",
			docs:         expire(fullSet(), 10, KindInsurance),
			wantCanRun:   false,
			wantBlocking: []string{"Insurance"},
		},
		{
			name:         "fitness expired: blocked, naming fitness and nothing else",
			docs:         expire(fullSet(), 1, KindFitness),
			wantCanRun:   false,
			wantBlocking: []string{"Fitness"},
		},
		{
			name:         "permit expired: blocked, naming the permit and nothing else",
			docs:         expire(fullSet(), 40, KindPermit),
			wantCanRun:   false,
			wantBlocking: []string{"Permit"},
		},
		{
			name:         "PUC expired: blocked, naming the PUC and nothing else",
			docs:         expire(fullSet(), 3, KindPUC),
			wantCanRun:   false,
			wantBlocking: []string{"PUC"},
		},
		{
			name:         "three expired at once: all three named",
			docs:         expire(fullSet(), 7, KindInsurance, KindPUC, KindFitness),
			wantCanRun:   false,
			wantBlocking: []string{"Fitness", "Insurance", "PUC"},
		},
		{
			name:         "all four blocking documents expired: all four named",
			docs:         expire(fullSet(), 2, KindInsurance, KindFitness, KindPermit, KindPUC),
			wantCanRun:   false,
			wantBlocking: []string{"Fitness", "Insurance", "PUC", "Permit"},
		},
		{
			name:         "RC expired: reported but the vehicle still runs by default",
			docs:         expire(fullSet(), 20, KindRC),
			wantCanRun:   true,
			wantReported: []string{"RC"},
		},
		{
			name:         "road tax expired: reported but the vehicle still runs by default",
			docs:         expire(fullSet(), 90, KindRoadTax),
			wantCanRun:   true,
			wantReported: []string{"RoadTax"},
		},
		{
			name:         "RC and road tax both expired: both reported, still runs",
			docs:         expire(fullSet(), 5, KindRC, KindRoadTax),
			wantCanRun:   true,
			wantReported: []string{"RC", "RoadTax"},
		},
		{
			name:         "RC switched on as blocking: an expired RC now stops the vehicle",
			docs:         expire(fullSet(), 20, KindRC),
			blocking:     map[DocKind]bool{KindInsurance: true, KindFitness: true, KindPermit: true, KindPUC: true, KindRC: true, KindRoadTax: false},
			wantCanRun:   false,
			wantBlocking: []string{"RC"},
		},
		{
			name:         "road tax switched on as blocking: an expired tax now stops the vehicle",
			docs:         expire(fullSet(), 11, KindRoadTax),
			blocking:     map[DocKind]bool{KindInsurance: true, KindFitness: true, KindPermit: true, KindPUC: true, KindRC: false, KindRoadTax: true},
			wantCanRun:   false,
			wantBlocking: []string{"RoadTax"},
		},
		{
			name:       "permit switched off for a permit-exempt carriage: an expired permit no longer blocks",
			docs:       expire(fullSet(), 30, KindPermit),
			blocking:   map[DocKind]bool{KindInsurance: true, KindFitness: true, KindPermit: false, KindPUC: true, KindRC: false, KindRoadTax: false},
			wantCanRun: true, wantReported: []string{"Permit"},
		},
		{
			name:        "insurance not on file at all: blocked, and said so",
			docs:        drop(fullSet(), KindInsurance),
			wantCanRun:  false,
			wantMissing: []string{"Insurance"},
		},
		{
			name:        "nothing on file at all: all four blocking kinds reported missing",
			docs:        nil,
			wantCanRun:  false,
			wantMissing: []string{"Fitness", "Insurance", "PUC", "Permit"},
		},
		{
			name:         "one expired and one missing are reported separately",
			docs:         expire(drop(fullSet(), KindPUC), 4, KindInsurance),
			wantCanRun:   false,
			wantBlocking: []string{"Insurance"},
			wantMissing:  []string{"PUC"},
		},
		{
			name:        "RC missing does not block, because RC does not block by default",
			docs:        drop(fullSet(), KindRC),
			wantCanRun:  true,
			wantMissing: nil,
		},
		{
			name:       "expiring today still runs: a document is valid through its expiry date",
			docs:       expire(fullSet(), 0, KindInsurance, KindFitness, KindPermit, KindPUC),
			wantCanRun: true,
		},
		{
			name:         "expired yesterday does not run",
			docs:         expire(fullSet(), 1, KindInsurance),
			wantCanRun:   false,
			wantBlocking: []string{"Insurance"},
		},
		{
			name:       "a driving licence record on the pile is ignored: it belongs to the driver",
			docs:       append(fullSet(), Document{DriverID: "D1", Kind: KindDrivingLicence, ExpiresOn: NewDate(2020, time.January, 1)}),
			wantCanRun: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanRunToday(lorry(), tc.docs, runDay, tc.blocking)

			if got.CanRun != tc.wantCanRun {
				t.Errorf("CanRun = %v, want %v (headline %q, detail %q)",
					got.CanRun, tc.wantCanRun, got.Headline, got.Detail)
			}
			assertKinds(t, "Blocking", labels(got.Blocking), tc.wantBlocking)
			assertKinds(t, "Missing", labels(got.Missing), tc.wantMissing)
			assertKinds(t, "Reported", labels(got.Reported), tc.wantReported)

			// Every blocker must carry a reason naming its own document, so the
			// owner knows what to go and renew.
			for _, item := range got.Blocking {
				if !strings.Contains(item.Reason, item.Label) {
					t.Errorf("blocking %s: reason %q does not name the document", item.Kind, item.Reason)
				}
				if item.Status.DaysOverdue < 1 {
					t.Errorf("blocking %s: DaysOverdue = %d, want at least 1", item.Kind, item.Status.DaysOverdue)
				}
			}
			if got.Headline == "" || got.Detail == "" {
				t.Errorf("headline/detail must always be filled, got %q / %q", got.Headline, got.Detail)
			}
			if got.CanRun && !strings.Contains(got.Headline, "can run today") {
				t.Errorf("headline for a runnable vehicle = %q", got.Headline)
			}
			if !got.CanRun && !strings.Contains(got.Headline, "must not run today") {
				t.Errorf("headline for a grounded vehicle = %q", got.Headline)
			}
		})
	}
}

func assertKinds(t *testing.T, field string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", field, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s = %v, want %v", field, got, want)
			return
		}
	}
}

// TestCanRunTodayNamesTheBlockerPrecisely is the headline promise: when exactly
// one document is expired, the answer names that one document and no other.
func TestCanRunTodayNamesTheBlockerPrecisely(t *testing.T) {
	for _, kind := range []DocKind{KindInsurance, KindFitness, KindPermit, KindPUC} {
		t.Run(string(kind), func(t *testing.T) {
			got := CanRunToday(lorry(), expire(fullSet(), 12, kind), runDay, nil)
			if got.CanRun {
				t.Fatalf("expired %s must ground the vehicle", kind)
			}
			if len(got.Blocking) != 1 {
				t.Fatalf("Blocking = %v, want exactly one entry", labels(got.Blocking))
			}
			blocker := got.Blocking[0]
			if blocker.Kind != kind {
				t.Errorf("blocker = %s, want %s", blocker.Kind, kind)
			}
			if blocker.Status.DaysOverdue != 12 {
				t.Errorf("DaysOverdue = %d, want 12", blocker.Status.DaysOverdue)
			}
			if !strings.Contains(got.Detail, kind.Label()) {
				t.Errorf("detail %q does not name %s", got.Detail, kind.Label())
			}
			for _, other := range []DocKind{KindInsurance, KindFitness, KindPermit, KindPUC} {
				if other == kind {
					continue
				}
				if strings.Contains(got.Detail, other.Label()) {
					t.Errorf("detail %q wrongly names %s too", got.Detail, other.Label())
				}
			}
		})
	}
}

// TestCanRunTodayOrdersBlockersByOverdue puts the longest-overdue document at
// the top, because that is the one that has been illegal the longest.
func TestCanRunTodayOrdersBlockersByOverdue(t *testing.T) {
	docs := fullSet()
	for i := range docs {
		switch docs[i].Kind {
		case KindPUC:
			docs[i].ExpiresOn = runDay.AddDays(-2)
		case KindInsurance:
			docs[i].ExpiresOn = runDay.AddDays(-45)
		case KindFitness:
			docs[i].ExpiresOn = runDay.AddDays(-9)
		}
	}
	got := CanRunToday(lorry(), docs, runDay, nil)
	want := []DocKind{KindInsurance, KindFitness, KindPUC}
	if len(got.Blocking) != len(want) {
		t.Fatalf("Blocking = %v, want %v", labels(got.Blocking), want)
	}
	for i, kind := range want {
		if got.Blocking[i].Kind != kind {
			t.Errorf("Blocking[%d] = %s, want %s", i, got.Blocking[i].Kind, kind)
		}
	}
}

// TestCanRunTodayListsExpiringWithoutBlocking proves a due-soon document warns
// but does not ground the vehicle.
func TestCanRunTodayListsExpiringWithoutBlocking(t *testing.T) {
	docs := fullSet()
	for i := range docs {
		if docs[i].Kind == KindPUC {
			docs[i].ExpiresOn = runDay.AddDays(5)
		}
	}
	got := CanRunToday(lorry(), docs, runDay, nil)
	if !got.CanRun {
		t.Fatalf("a document due in 5 days must not ground the vehicle: %+v", got.Blocking)
	}
	if len(got.Expiring) != 1 || got.Expiring[0].Kind != KindPUC {
		t.Fatalf("Expiring = %v, want [PUC]", labels(got.Expiring))
	}
	if got.Expiring[0].Status.Tier != TierWarnDays {
		t.Errorf("Tier = %d, want %d", got.Expiring[0].Status.Tier, TierWarnDays)
	}
}

func TestDefaultBlockingIsTheStatutoryFour(t *testing.T) {
	blocking := DefaultBlocking()
	for _, kind := range []DocKind{KindInsurance, KindFitness, KindPermit, KindPUC} {
		if !blocking[kind] {
			t.Errorf("%s must block by default", kind)
		}
	}
	for _, kind := range []DocKind{KindRC, KindRoadTax} {
		if blocking[kind] {
			t.Errorf("%s must be reported but not block by default", kind)
		}
	}
}

func TestFleetCanRunToday(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(lorry()); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	for _, doc := range expire(fullSet(), 6, KindFitness) {
		if _, err := f.PutDocument(doc); err != nil {
			t.Fatalf("PutDocument %s: %v", doc.Kind, err)
		}
	}

	got, ok := f.CanRunToday("ka 01 ab 1234", runDay)
	if !ok {
		t.Fatal("the vehicle should be found however the registration number is typed")
	}
	if got.CanRun {
		t.Error("an expired certificate of fitness must ground the vehicle")
	}
	if len(got.Blocking) != 1 || got.Blocking[0].Kind != KindFitness {
		t.Errorf("Blocking = %v, want [Fitness]", labels(got.Blocking))
	}

	if _, ok := f.CanRunToday("KA99ZZ0000", runDay); ok {
		t.Error("an unknown registration number must not be answered")
	}
}

func TestCanRunTodaySwitchingPolicyFlipsTheAnswer(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(lorry()); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	for _, doc := range expire(fullSet(), 15, KindRoadTax) {
		if _, err := f.PutDocument(doc); err != nil {
			t.Fatalf("PutDocument %s: %v", doc.Kind, err)
		}
	}

	before, _ := f.CanRunToday(lorry().RegNo, runDay)
	if !before.CanRun {
		t.Fatalf("road tax must not block by default, got %v", labels(before.Blocking))
	}
	if err := f.SetBlocking(KindRoadTax, true); err != nil {
		t.Fatalf("SetBlocking: %v", err)
	}
	after, _ := f.CanRunToday(lorry().RegNo, runDay)
	if after.CanRun {
		t.Fatal("with road tax set to block, an expired tax must ground the vehicle")
	}
	if len(after.Blocking) != 1 || after.Blocking[0].Kind != KindRoadTax {
		t.Errorf("Blocking = %v, want [RoadTax]", labels(after.Blocking))
	}
	if err := f.SetBlocking(KindDrivingLicence, true); err == nil {
		t.Error("a driving licence is not a vehicle document and must be refused by the vehicle policy")
	}
}
