package fleet

import (
	"testing"
	"time"
)

// boardFleet builds a two-lorry, two-driver fleet with a deliberately messy
// spread of expiry dates around 1 March 2027.
func boardFleet(t *testing.T) *Fleet {
	t.Helper()
	f := New()
	vehicles := []Vehicle{
		{RegNo: "KA01AB1234", Type: TypeGoods, Make: "Tata 407", OwnerName: "Ravi Transport"},
		{RegNo: "MH12CD5678", Type: TypePassenger, Make: "Force Traveller", OwnerName: "Ravi Transport"},
	}
	for _, v := range vehicles {
		if _, err := f.PutVehicle(v); err != nil {
			t.Fatalf("PutVehicle %s: %v", v.RegNo, err)
		}
	}
	drivers := []Driver{{Name: "Suresh"}, {Name: "Imran"}}
	for i, d := range drivers {
		saved, err := f.PutDriver(d)
		if err != nil {
			t.Fatalf("PutDriver %s: %v", d.Name, err)
		}
		drivers[i] = saved
	}
	docs := []Document{
		{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, Number: "POL-1", ExpiresOn: NewDate(2027, time.February, 10)}, // 19 days overdue
		{VehicleRegNo: "KA01AB1234", Kind: KindPUC, Number: "PUC-1", ExpiresOn: NewDate(2027, time.March, 10)},          // 9 days left
		{VehicleRegNo: "KA01AB1234", Kind: KindFitness, Number: "CF-1", ExpiresOn: NewDate(2028, time.July, 1)},         // valid
		{VehicleRegNo: "MH12CD5678", Kind: KindPermit, Number: "PRM-1", ExpiresOn: NewDate(2026, time.December, 1)},     // 90 days overdue
		{VehicleRegNo: "MH12CD5678", Kind: KindPUC, Number: "PUC-2", ExpiresOn: NewDate(2027, time.March, 1)},           // expires today
		{DriverID: drivers[0].ID, Kind: KindDrivingLicence, Number: "DL-1", ExpiresOn: NewDate(2027, time.March, 20)},   // 19 days left
		{DriverID: drivers[1].ID, Kind: KindDrivingLicence, Number: "DL-2", ExpiresOn: NewDate(2027, time.January, 5)},  // 55 days overdue
	}
	for _, doc := range docs {
		if _, err := f.PutDocument(doc); err != nil {
			t.Fatalf("PutDocument %s: %v", doc.Kind, err)
		}
	}
	return f
}

func TestFleetBoardSortsByUrgency(t *testing.T) {
	f := boardFleet(t)
	asOf := NewDate(2027, time.March, 1)
	rows := f.FleetBoard(asOf)

	if len(rows) != 7 {
		t.Fatalf("board has %d rows, want 7", len(rows))
	}

	// Expired first, longest overdue at the top; then the soonest to expire.
	want := []struct {
		subject string
		kind    DocKind
		status  StatusKind
	}{
		{"MH 12 CD 5678", KindPermit, StatusExpired},    // 90 days overdue
		{"Imran", KindDrivingLicence, StatusExpired},    // 55 days overdue
		{"KA 01 AB 1234", KindInsurance, StatusExpired}, // 19 days overdue
		{"MH 12 CD 5678", KindPUC, StatusDueSoon},       // expires today
		{"KA 01 AB 1234", KindPUC, StatusDueSoon},       // 9 days left
		{"Suresh", KindDrivingLicence, StatusDueSoon},   // 19 days left
		{"KA 01 AB 1234", KindFitness, StatusValid},     // far out
	}
	for i, w := range want {
		if rows[i].SubjectLabel != w.subject || rows[i].Kind != w.kind || rows[i].Status.Kind != w.status {
			t.Errorf("row %d = %s / %s / %s, want %s / %s / %s",
				i, rows[i].SubjectLabel, rows[i].Kind, rows[i].Status.Kind, w.subject, w.kind, w.status)
		}
	}

	// The board must never be ordered loosely: each expiry date is on or after
	// the one above it.
	for i := 1; i < len(rows); i++ {
		if rows[i].ExpiresOn.Before(rows[i-1].ExpiresOn) {
			t.Errorf("row %d (%s) expires before row %d (%s)",
				i, rows[i].ExpiresOn, i-1, rows[i-1].ExpiresOn)
		}
	}
}

func TestFleetBoardCarriesTheWholeRow(t *testing.T) {
	f := boardFleet(t)
	rows := f.FleetBoard(NewDate(2027, time.March, 1))

	var insurance BoardRow
	for _, row := range rows {
		if row.Kind == KindInsurance {
			insurance = row
		}
	}
	if insurance.SubjectKind != SubjectVehicle {
		t.Errorf("SubjectKind = %q, want %q", insurance.SubjectKind, SubjectVehicle)
	}
	if insurance.SubjectKey != "KA01AB1234" {
		t.Errorf("SubjectKey = %q, want the normalised registration number", insurance.SubjectKey)
	}
	if insurance.Detail != "Tata 407" {
		t.Errorf("Detail = %q, want the make so the board needs no second lookup", insurance.Detail)
	}
	if insurance.Number != "POL-1" || insurance.Label != "Insurance" {
		t.Errorf("Number/Label = %q/%q, want POL-1/Insurance", insurance.Number, insurance.Label)
	}
	if insurance.Status.DaysOverdue != 19 {
		t.Errorf("DaysOverdue = %d, want 19", insurance.Status.DaysOverdue)
	}

	var licence BoardRow
	for _, row := range rows {
		if row.Kind == KindDrivingLicence && row.Number == "DL-1" {
			licence = row
		}
	}
	if licence.SubjectKind != SubjectDriver {
		t.Errorf("SubjectKind = %q, want %q", licence.SubjectKind, SubjectDriver)
	}
	if licence.SubjectLabel != "Suresh" {
		t.Errorf("SubjectLabel = %q, want the driver's name", licence.SubjectLabel)
	}
}

func TestFleetBoardEmptyFleet(t *testing.T) {
	f := New()
	rows := f.FleetBoard(NewDate(2027, time.March, 1))
	if len(rows) != 0 {
		t.Fatalf("an empty fleet must give an empty board, got %d rows", len(rows))
	}
	if got := Summarise(rows); got != (Counts{}) {
		t.Errorf("Summarise = %+v, want all zeroes", got)
	}
}

func TestFleetBoardUndatedRowsSortLast(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypeGoods}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	if _, err := f.PutDocument(Document{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, ExpiresOn: NewDate(2030, time.January, 1)}); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}
	// PutDocument refuses an undated document, so this is the only way such a
	// row can exist: a hand-edited data file. The board must still cope.
	f.Documents = append(f.Documents, Document{VehicleRegNo: "KA01AB1234", Kind: KindPermit})

	rows := f.FleetBoard(NewDate(2027, time.March, 1))
	if len(rows) != 2 {
		t.Fatalf("board has %d rows, want 2", len(rows))
	}
	if rows[1].Kind != KindPermit || rows[1].Status.Kind != StatusUnknown {
		t.Errorf("last row = %s / %s, want Permit / unknown", rows[1].Kind, rows[1].Status.Kind)
	}
	if got := Summarise(rows); got.Unknown != 1 || got.Valid != 1 || got.Total != 2 {
		t.Errorf("Summarise = %+v, want 1 valid, 1 unknown, 2 total", got)
	}
}

func TestSummarise(t *testing.T) {
	f := boardFleet(t)
	got := Summarise(f.FleetBoard(NewDate(2027, time.March, 1)))
	want := Counts{Total: 7, Expired: 3, DueSoon: 3, Valid: 1}
	if got != want {
		t.Errorf("Summarise = %+v, want %+v", got, want)
	}
}

func TestLicenceBoard(t *testing.T) {
	f := boardFleet(t)
	rows := f.LicenceBoard(NewDate(2027, time.March, 1))
	if len(rows) != 2 {
		t.Fatalf("licence board has %d rows, want 2", len(rows))
	}
	if rows[0].SubjectLabel != "Imran" || rows[0].Status.Kind != StatusExpired {
		t.Errorf("first licence row = %s / %s, want Imran / expired", rows[0].SubjectLabel, rows[0].Status.Kind)
	}
	for _, row := range rows {
		if row.Kind != KindDrivingLicence {
			t.Errorf("licence board contains a %s row", row.Kind)
		}
		if row.SubjectKind != SubjectDriver {
			t.Errorf("licence row for %s has subject kind %q", row.SubjectLabel, row.SubjectKind)
		}
	}
}

func TestVehicleBoard(t *testing.T) {
	f := boardFleet(t)
	rows := f.VehicleBoard("ka-01-ab-1234", NewDate(2027, time.March, 1))
	if len(rows) != 3 {
		t.Fatalf("vehicle board has %d rows, want 3", len(rows))
	}
	wantKinds := []DocKind{KindInsurance, KindPUC, KindFitness}
	for i, kind := range wantKinds {
		if rows[i].Kind != kind {
			t.Errorf("row %d = %s, want %s", i, rows[i].Kind, kind)
		}
	}
}

func TestNextDue(t *testing.T) {
	tests := []struct {
		name      string
		docs      []Document
		wantFound bool
		wantKind  DocKind
		wantDate  Date
	}{
		{
			name:      "no documents",
			docs:      nil,
			wantFound: false,
		},
		{
			name: "the soonest future expiry wins",
			docs: []Document{
				{Kind: KindFitness, ExpiresOn: NewDate(2028, time.July, 1)},
				{Kind: KindPUC, ExpiresOn: NewDate(2027, time.March, 10)},
				{Kind: KindInsurance, ExpiresOn: NewDate(2027, time.December, 31)},
			},
			wantFound: true, wantKind: KindPUC, wantDate: NewDate(2027, time.March, 10),
		},
		{
			name: "an already-expired document is the most pressing of all",
			docs: []Document{
				{Kind: KindPUC, ExpiresOn: NewDate(2027, time.March, 10)},
				{Kind: KindInsurance, ExpiresOn: NewDate(2026, time.November, 2)},
			},
			wantFound: true, wantKind: KindInsurance, wantDate: NewDate(2026, time.November, 2),
		},
		{
			name: "two on the same day break the tie on document order, every time",
			docs: []Document{
				{Kind: KindPUC, ExpiresOn: NewDate(2027, time.April, 1)},
				{Kind: KindInsurance, ExpiresOn: NewDate(2027, time.April, 1)},
				{Kind: KindPermit, ExpiresOn: NewDate(2027, time.April, 1)},
			},
			wantFound: true, wantKind: KindInsurance, wantDate: NewDate(2027, time.April, 1),
		},
		{
			name: "undated documents are skipped",
			docs: []Document{
				{Kind: KindRC},
				{Kind: KindRoadTax},
				{Kind: KindPermit, ExpiresOn: NewDate(2029, time.January, 1)},
			},
			wantFound: true, wantKind: KindPermit, wantDate: NewDate(2029, time.January, 1),
		},
		{
			name:      "every document undated",
			docs:      []Document{{Kind: KindRC}, {Kind: KindRoadTax}},
			wantFound: false,
		},
		{
			name:      "a single document is its own answer",
			docs:      []Document{{Kind: KindFitness, ExpiresOn: NewDate(2027, time.June, 30)}},
			wantFound: true, wantKind: KindFitness, wantDate: NewDate(2027, time.June, 30),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := NextDue(tc.docs)
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v", found, tc.wantFound)
			}
			if !found {
				return
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %s, want %s", got.Kind, tc.wantKind)
			}
			if got.ExpiresOn != tc.wantDate {
				t.Errorf("ExpiresOn = %s, want %s", got.ExpiresOn, tc.wantDate)
			}
		})
	}
}

func TestNextDueIsStableWhateverTheInputOrder(t *testing.T) {
	a := []Document{
		{Kind: KindPermit, ExpiresOn: NewDate(2027, time.April, 1)},
		{Kind: KindInsurance, ExpiresOn: NewDate(2027, time.April, 1)},
	}
	b := []Document{
		{Kind: KindInsurance, ExpiresOn: NewDate(2027, time.April, 1)},
		{Kind: KindPermit, ExpiresOn: NewDate(2027, time.April, 1)},
	}
	first, _ := NextDue(a)
	second, _ := NextDue(b)
	if first.Kind != second.Kind {
		t.Errorf("NextDue gave %s then %s for the same records in a different order", first.Kind, second.Kind)
	}
}

func TestFleetNextDue(t *testing.T) {
	f := boardFleet(t)
	got, found := f.NextDue("KA01AB1234")
	if !found {
		t.Fatal("the lorry has documents, so it has a next due date")
	}
	if got.Kind != KindInsurance || got.ExpiresOn != NewDate(2027, time.February, 10) {
		t.Errorf("NextDue = %s on %s, want Insurance on 2027-02-10", got.Kind, got.ExpiresOn)
	}
	if _, found := f.NextDue("KA99ZZ0000"); found {
		t.Error("an unknown vehicle has no next due date")
	}
}
