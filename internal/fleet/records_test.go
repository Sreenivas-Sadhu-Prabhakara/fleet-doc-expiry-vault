package fleet

import (
	"strings"
	"testing"
	"time"
)

func TestNormaliseRegNo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"KA01AB1234", "KA01AB1234"},
		{"ka01ab1234", "KA01AB1234"},
		{"KA 01 AB 1234", "KA01AB1234"},
		{"ka-01-ab-1234", "KA01AB1234"},
		{" KA01 AB1234 ", "KA01AB1234"},
		{"KA.01.AB.1234", "KA01AB1234"},
		{"", ""},
		{"   ", ""},
		{"---", ""},
	}
	for _, tc := range tests {
		if got := NormaliseRegNo(tc.in); got != tc.want {
			t.Errorf("NormaliseRegNo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrettyRegNo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"KA01AB1234", "KA 01 AB 1234"},
		{"ka 01 ab 1234", "KA 01 AB 1234"},
		{"MH12CD5678", "MH 12 CD 5678"},
		{"DL1CAB1234", "DL 1 CAB 1234"},
		{"KA011234", "KA 011234"},
		// Anything that does not fall into two to four runs is left alone
		// rather than mangled into a shape it is not.
		{"KA", "KA"},
		{"KA01AB1234X9Z", "KA01AB1234X9Z"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := PrettyRegNo(tc.in); got != tc.want {
			t.Errorf("PrettyRegNo(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDocKindLabels(t *testing.T) {
	tests := map[DocKind]string{
		KindRC:             "Registration (RC)",
		KindInsurance:      "Insurance",
		KindFitness:        "Fitness (CF)",
		KindPermit:         "Permit",
		KindPUC:            "PUC",
		KindRoadTax:        "Road tax",
		KindDrivingLicence: "Driving licence",
	}
	for kind, want := range tests {
		if got := kind.Label(); got != want {
			t.Errorf("%s.Label() = %q, want %q", kind, got, want)
		}
	}
	if len(tests) != len(AllKinds()) {
		t.Errorf("there are %d kinds but %d labels under test", len(AllKinds()), len(tests))
	}
}

func TestBelongsToDriver(t *testing.T) {
	if !KindDrivingLicence.BelongsToDriver() {
		t.Error("a driving licence belongs to the driver")
	}
	for _, kind := range VehicleKinds() {
		if kind.BelongsToDriver() {
			t.Errorf("%s belongs to the vehicle, not the driver", kind)
		}
	}
	if len(VehicleKinds())+len(DriverKinds()) != len(AllKinds()) {
		t.Error("every kind must belong either to a vehicle or to a driver")
	}
}

func TestParseVehicleType(t *testing.T) {
	tests := []struct {
		in      string
		want    VehicleType
		wantErr bool
	}{
		{in: "goods", want: TypeGoods},
		{in: "GOODS", want: TypeGoods},
		{in: " truck ", want: TypeGoods},
		{in: "passenger", want: TypePassenger},
		{in: "bus", want: TypePassenger},
		{in: "", wantErr: true},
		{in: "tractor", wantErr: true},
	}
	for _, tc := range tests {
		got, err := ParseVehicleType(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseVehicleType(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVehicleType(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseVehicleType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPutVehicle(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: " ka-01-ab-1234 ", Type: TypeGoods, Make: " Tata 407 ", OwnerName: " Ravi "}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	if len(f.Vehicles) != 1 {
		t.Fatalf("fleet has %d vehicles, want 1", len(f.Vehicles))
	}
	got := f.Vehicles[0]
	if got.RegNo != "KA01AB1234" || got.Make != "Tata 407" || got.OwnerName != "Ravi" {
		t.Errorf("stored %+v, want trimmed and normalised values", got)
	}

	// Saving the same registration number again updates the one record.
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypePassenger, Make: "Force", OwnerName: "Ravi"}); err != nil {
		t.Fatalf("PutVehicle (update): %v", err)
	}
	if len(f.Vehicles) != 1 {
		t.Errorf("fleet has %d vehicles after an update, want 1", len(f.Vehicles))
	}
	if f.Vehicles[0].Type != TypePassenger {
		t.Errorf("Type = %q, want the updated value", f.Vehicles[0].Type)
	}

	if _, err := f.PutVehicle(Vehicle{RegNo: "", Type: TypeGoods}); err == nil {
		t.Error("a vehicle with no registration number must be refused")
	}
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA02XY1111"}); err == nil {
		t.Error("a vehicle with no type must be refused")
	}
}

func TestPutDocumentValidation(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypeGoods}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	driver, err := f.PutDriver(Driver{Name: "Suresh"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}

	tests := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{
			name: "a good vehicle document",
			doc:  Document{VehicleRegNo: "ka 01 ab 1234", Kind: KindPUC, ExpiresOn: NewDate(2027, time.October, 2)},
		},
		{
			name: "a good driving licence",
			doc:  Document{DriverID: driver.ID, Kind: KindDrivingLicence, ExpiresOn: NewDate(2041, time.May, 9)},
		},
		{
			name:    "no expiry date",
			doc:     Document{VehicleRegNo: "KA01AB1234", Kind: KindInsurance},
			wantErr: "an expiry date is required",
		},
		{
			name:    "expiry before issue",
			doc:     Document{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, IssuedOn: NewDate(2027, time.April, 1), ExpiresOn: NewDate(2027, time.March, 1)},
			wantErr: "falls before the issue date",
		},
		{
			name:    "unknown vehicle",
			doc:     Document{VehicleRegNo: "KA99ZZ0000", Kind: KindInsurance, ExpiresOn: NewDate(2028, time.March, 31)},
			wantErr: "no vehicle with registration",
		},
		{
			name:    "unknown driver",
			doc:     Document{DriverID: "D99", Kind: KindDrivingLicence, ExpiresOn: NewDate(2041, time.May, 9)},
			wantErr: "no driver with ID",
		},
		{
			name:    "unknown kind",
			doc:     Document{VehicleRegNo: "KA01AB1234", Kind: "Toll Pass", ExpiresOn: NewDate(2028, time.March, 31)},
			wantErr: "is not one of RC",
		},
		{
			name:    "a vehicle document aimed at a driver",
			doc:     Document{DriverID: driver.ID, Kind: KindInsurance, ExpiresOn: NewDate(2028, time.March, 31)},
			wantErr: "no vehicle with registration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.PutDocument(tc.doc)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("PutDocument: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("PutDocument returned no error, want one mentioning %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestPutDocumentReplacesByKind(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypeGoods}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	old := Document{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, Number: "OLD", ExpiresOn: NewDate(2027, time.March, 31)}
	renewed := Document{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, Number: "NEW", ExpiresOn: NewDate(2028, time.March, 31)}
	if _, err := f.PutDocument(old); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}
	if _, err := f.PutDocument(renewed); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}
	if len(f.Documents) != 1 {
		t.Fatalf("fleet holds %d documents, want 1 current insurance record", len(f.Documents))
	}
	if f.Documents[0].Number != "NEW" {
		t.Errorf("Number = %q, want NEW", f.Documents[0].Number)
	}
}

func TestRemoveVehicleTakesItsDocumentsWithIt(t *testing.T) {
	f := New()
	for _, reg := range []string{"KA01AB1234", "MH12CD5678"} {
		if _, err := f.PutVehicle(Vehicle{RegNo: reg, Type: TypeGoods}); err != nil {
			t.Fatalf("PutVehicle: %v", err)
		}
		if _, err := f.PutDocument(Document{VehicleRegNo: reg, Kind: KindPUC, ExpiresOn: NewDate(2027, time.October, 2)}); err != nil {
			t.Fatalf("PutDocument: %v", err)
		}
	}
	driver, err := f.PutDriver(Driver{Name: "Suresh"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	if _, err := f.PutDocument(Document{DriverID: driver.ID, Kind: KindDrivingLicence, ExpiresOn: NewDate(2041, time.May, 9)}); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}

	if !f.RemoveVehicle("ka 01 ab 1234") {
		t.Fatal("RemoveVehicle returned false for a vehicle that is on file")
	}
	if len(f.Vehicles) != 1 || f.Vehicles[0].RegNo != "MH12CD5678" {
		t.Errorf("vehicles = %+v, want only MH12CD5678", f.Vehicles)
	}
	if len(f.Documents) != 2 {
		t.Errorf("documents = %+v, want the other lorry's PUC and the licence", f.Documents)
	}
	for _, doc := range f.Documents {
		if doc.VehicleRegNo == "KA01AB1234" {
			t.Error("a removed vehicle must not leave documents behind")
		}
	}
	if f.RemoveVehicle("KA01AB1234") {
		t.Error("removing the same vehicle twice must report false the second time")
	}
}

func TestRemoveDriverTakesTheLicenceWithIt(t *testing.T) {
	f := New()
	driver, err := f.PutDriver(Driver{Name: "Suresh", Phone: "9876500000"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	if _, err := f.PutDocument(Document{DriverID: driver.ID, Kind: KindDrivingLicence, ExpiresOn: NewDate(2041, time.May, 9)}); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}
	if !f.RemoveDriver(driver.ID) {
		t.Fatal("RemoveDriver returned false for a driver on file")
	}
	if len(f.Drivers) != 0 || len(f.Documents) != 0 {
		t.Errorf("drivers = %+v, documents = %+v, want both empty", f.Drivers, f.Documents)
	}
	if f.RemoveDriver(driver.ID) {
		t.Error("removing the same driver twice must report false the second time")
	}
}

func TestRemoveDocument(t *testing.T) {
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypeGoods}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	for _, kind := range []DocKind{KindPUC, KindInsurance} {
		if _, err := f.PutDocument(Document{VehicleRegNo: "KA01AB1234", Kind: kind, ExpiresOn: NewDate(2028, time.March, 31)}); err != nil {
			t.Fatalf("PutDocument: %v", err)
		}
	}
	if !f.RemoveDocument("ka-01-ab-1234", KindPUC) {
		t.Fatal("RemoveDocument returned false for a document on file")
	}
	if len(f.Documents) != 1 || f.Documents[0].Kind != KindInsurance {
		t.Errorf("documents = %+v, want only the insurance record", f.Documents)
	}
	if f.RemoveDocument("KA01AB1234", KindPUC) {
		t.Error("removing the same document twice must report false the second time")
	}
}

func TestPutDriverGeneratesIDs(t *testing.T) {
	f := New()
	first, err := f.PutDriver(Driver{Name: "Suresh"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	second, err := f.PutDriver(Driver{Name: "Imran"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	if first.ID == "" || second.ID == "" || first.ID == second.ID {
		t.Fatalf("IDs = %q and %q, want two distinct non-empty IDs", first.ID, second.ID)
	}

	// Removing the first driver must not make the next new driver reuse its ID,
	// or a stale document would silently attach to the wrong person.
	if !f.RemoveDriver(first.ID) {
		t.Fatal("RemoveDriver: expected the driver to be on file")
	}
	third, err := f.PutDriver(Driver{Name: "Anil"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	if third.ID == first.ID || third.ID == second.ID {
		t.Errorf("new driver ID = %q, want it not to reuse %q or %q", third.ID, first.ID, second.ID)
	}

	if _, err := f.PutDriver(Driver{Name: "  "}); err == nil {
		t.Error("a driver with no name must be refused")
	}
}

func TestNormaliseFillsInMissingPolicy(t *testing.T) {
	// A data file written before a policy key existed, or edited by hand.
	f := &Fleet{
		Vehicles:  []Vehicle{{RegNo: "ka 01 ab 1234", Type: TypeGoods}},
		Documents: []Document{{VehicleRegNo: "ka-01-ab-1234", Kind: KindPUC, ExpiresOn: NewDate(2027, time.October, 2)}},
		Blocking:  map[DocKind]bool{KindInsurance: false},
	}
	f.Normalise()

	if f.Vehicles[0].RegNo != "KA01AB1234" || f.Documents[0].VehicleRegNo != "KA01AB1234" {
		t.Errorf("registration numbers not normalised: %q / %q", f.Vehicles[0].RegNo, f.Documents[0].VehicleRegNo)
	}
	// An explicit choice already in the file must be kept, not overwritten.
	if f.Blocking[KindInsurance] {
		t.Error("Normalise must not overwrite a policy choice already in the file")
	}
	// The keys that were missing must be filled from the defaults.
	if !f.Blocking[KindFitness] || !f.Blocking[KindPUC] || !f.Blocking[KindPermit] {
		t.Errorf("Blocking = %+v, want the missing keys filled from the defaults", f.Blocking)
	}
	if f.Blocking[KindRC] || f.Blocking[KindRoadTax] {
		t.Errorf("Blocking = %+v, want RC and road tax non-blocking by default", f.Blocking)
	}
	if got := f.Validity[KindPUC]; got.RenewalMonths != PUCValidityMonths {
		t.Errorf("Validity[PUC] = %+v, want the default table", got)
	}
}

func TestSubjectKey(t *testing.T) {
	vehicleDoc := Document{VehicleRegNo: "KA01AB1234", DriverID: "D1", Kind: KindInsurance}
	if got := vehicleDoc.SubjectKey(); got != "KA01AB1234" {
		t.Errorf("SubjectKey = %q, want the registration number", got)
	}
	licence := Document{VehicleRegNo: "KA01AB1234", DriverID: "D1", Kind: KindDrivingLicence}
	if got := licence.SubjectKey(); got != "D1" {
		t.Errorf("SubjectKey = %q, want the driver ID", got)
	}
}
