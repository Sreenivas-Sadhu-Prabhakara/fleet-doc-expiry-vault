package fleet

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func csvFleet(t *testing.T) *Fleet {
	t.Helper()
	f := New()
	if _, err := f.PutVehicle(Vehicle{RegNo: "KA01AB1234", Type: TypeGoods, Make: "Tata 407", OwnerName: "Ravi Transport"}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	if _, err := f.PutVehicle(Vehicle{RegNo: "MH12CD5678", Type: TypePassenger, Make: "Force Traveller", OwnerName: "Ravi Transport"}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	driver, err := f.PutDriver(Driver{Name: "Suresh Kumar", Phone: "9876500000"})
	if err != nil {
		t.Fatalf("PutDriver: %v", err)
	}
	docs := []Document{
		{VehicleRegNo: "KA01AB1234", Kind: KindInsurance, Number: "POL/2027/88", IssuedOn: NewDate(2027, time.April, 1), ExpiresOn: NewDate(2028, time.March, 31), FileRef: "scans/ka01-insurance.pdf"},
		{VehicleRegNo: "KA01AB1234", Kind: KindPUC, Number: "PUC-77", IssuedOn: NewDate(2027, time.April, 2), ExpiresOn: NewDate(2027, time.October, 2), FileRef: "scans/ka01-puc.jpg"},
		{VehicleRegNo: "MH12CD5678", Kind: KindPermit, Number: "PRM/9", IssuedOn: NewDate(2026, time.January, 1), ExpiresOn: NewDate(2031, time.January, 1)},
		{DriverID: driver.ID, Kind: KindDrivingLicence, Number: "KA0120110012345", IssuedOn: NewDate(2021, time.May, 10), ExpiresOn: NewDate(2041, time.May, 9), FileRef: "scans/suresh-dl.pdf"},
	}
	for _, doc := range docs {
		if _, err := f.PutDocument(doc); err != nil {
			t.Fatalf("PutDocument %s: %v", doc.Kind, err)
		}
	}
	return f
}

func TestCSVRoundTrip(t *testing.T) {
	original := csvFleet(t)

	var buf bytes.Buffer
	if err := WriteCSV(&buf, original); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	header := strings.SplitN(buf.String(), "\n", 2)[0]
	if header != strings.Join(CSVHeader, ",") {
		t.Errorf("header = %q, want %q", header, strings.Join(CSVHeader, ","))
	}

	restored, warnings, err := ReadCSV(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("a file the vault wrote itself must import without warnings, got %v", warnings)
	}

	if len(restored.Vehicles) != len(original.Vehicles) {
		t.Fatalf("restored %d vehicles, want %d", len(restored.Vehicles), len(original.Vehicles))
	}
	for i, want := range original.Vehicles {
		if restored.Vehicles[i] != want {
			t.Errorf("vehicle %d = %+v, want %+v", i, restored.Vehicles[i], want)
		}
	}
	if len(restored.Drivers) != len(original.Drivers) {
		t.Fatalf("restored %d drivers, want %d", len(restored.Drivers), len(original.Drivers))
	}
	for i, want := range original.Drivers {
		if restored.Drivers[i] != want {
			t.Errorf("driver %d = %+v, want %+v", i, restored.Drivers[i], want)
		}
	}
	if len(restored.Documents) != len(original.Documents) {
		t.Fatalf("restored %d documents, want %d", len(restored.Documents), len(original.Documents))
	}
	for i, want := range original.Documents {
		if restored.Documents[i] != want {
			t.Errorf("document %d = %+v, want %+v", i, restored.Documents[i], want)
		}
	}

	// A board built from the restored records must read identically.
	asOf := NewDate(2027, time.June, 1)
	before := original.FleetBoard(asOf)
	after := restored.FleetBoard(asOf)
	if len(before) != len(after) {
		t.Fatalf("board length changed across the round trip: %d then %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("board row %d changed across the round trip:\n before %+v\n after  %+v", i, before[i], after[i])
		}
	}
}

func TestCSVExportIsOpenableInASpreadsheet(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, csvFleet(t)); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	// header + 2 vehicles + 1 driver + 4 documents
	if len(lines) != 8 {
		t.Fatalf("export has %d lines, want 8:\n%s", len(lines), buf.String())
	}
	width := len(CSVHeader)
	for i, line := range lines {
		if got := len(strings.Split(strings.TrimRight(line, "\r"), ",")); got != width {
			t.Errorf("line %d has %d columns, want %d: %s", i, got, width, line)
		}
	}
	if !strings.Contains(buf.String(), "2028-03-31") {
		t.Error("dates must export in the canonical YYYY-MM-DD form")
	}
}

func TestReadCSV(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantErr      bool
		wantVehicles int
		wantDrivers  int
		wantDocs     int
		wantWarnings []string // substrings, one per expected warning
	}{
		{
			name: "a minimal hand-typed file",
			in: `record_type,reg_no,vehicle_type,make,owner_name,driver_id,driver_name,driver_phone,doc_kind,doc_number,issued_on,expires_on,file_ref
vehicle,KA01AB1234,goods,Tata 407,Ravi,,,,,,,,
document,KA01AB1234,,,,,,,Insurance,POL-1,2027-04-01,2028-03-31,
`,
			wantVehicles: 1, wantDocs: 1,
		},
		{
			name: "columns in a different order, headers with spaces and capitals",
			in: `Doc Kind,Record Type,Expires On,Reg No,Vehicle Type,Doc Number
,vehicle,,KA01AB1234,goods,
Insurance,document,31-03-2028,ka 01 ab 1234,,POL-1
`,
			wantVehicles: 1, wantDocs: 1,
		},
		{
			name: "documents before the vehicle they belong to still attach",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
document,KA01AB1234,,PUC,2027-10-02
vehicle,KA01AB1234,goods,,
`,
			wantVehicles: 1, wantDocs: 1,
		},
		{
			name: "blank rows between records are skipped",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
,,,,
document,KA01AB1234,,PUC,2027-10-02
`,
			wantVehicles: 1, wantDocs: 1,
		},
		{
			name: "a driving licence attaches to the driver, not to a vehicle",
			in: `record_type,driver_id,driver_name,driver_phone,doc_kind,doc_number,expires_on
driver,D1,Suresh,9876500000,,,
document,D1,,,DrivingLicence,KA0120110012345,2041-05-09
`,
			wantDrivers: 1, wantDocs: 1,
		},
		{
			name: "a document for a vehicle that is not on file is warned about, not silently dropped",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
document,MH12CD5678,,PUC,2027-10-02
`,
			wantVehicles: 1, wantDocs: 0,
			wantWarnings: []string{"no vehicle with registration"},
		},
		{
			name: "a bad expiry date is warned about and the rest of the file still imports",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
document,KA01AB1234,,PUC,whenever
document,KA01AB1234,,Insurance,2028-03-31
`,
			wantVehicles: 1, wantDocs: 1,
			wantWarnings: []string{"expiry date"},
		},
		{
			name: "a document with no expiry date is warned about",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
document,KA01AB1234,,PUC,
`,
			wantVehicles: 1, wantDocs: 0,
			wantWarnings: []string{"no expiry date"},
		},
		{
			name: "an unknown document kind is warned about",
			in: `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
document,KA01AB1234,,Toll Pass,2027-10-02
`,
			wantVehicles: 1, wantDocs: 0,
			wantWarnings: []string{"not one of RC"},
		},
		{
			name: "an unknown record type is warned about",
			in: `record_type,reg_no,vehicle_type
trailer,KA01AB1234,goods
`,
			wantWarnings: []string{"is not vehicle, driver or document"},
		},
		{
			name: "a vehicle with no type is warned about",
			in: `record_type,reg_no,vehicle_type
vehicle,KA01AB1234,
`,
			wantWarnings: []string{"goods"},
		},
		{
			name: "an expiry before the issue date is refused",
			in: `record_type,reg_no,vehicle_type,doc_kind,issued_on,expires_on
vehicle,KA01AB1234,goods,,,
document,KA01AB1234,,PUC,2027-10-02,2027-04-02
`,
			wantVehicles: 1, wantDocs: 0,
			wantWarnings: []string{"falls before the issue date"},
		},
		{
			name:    "no header",
			in:      "KA01AB1234,goods\n",
			wantErr: true,
		},
		{
			name:    "empty file",
			in:      "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, warnings, err := ReadCSV(strings.NewReader(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatal("ReadCSV returned no error, want one")
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadCSV: %v", err)
			}
			if len(got.Vehicles) != tc.wantVehicles {
				t.Errorf("imported %d vehicles, want %d", len(got.Vehicles), tc.wantVehicles)
			}
			if len(got.Drivers) != tc.wantDrivers {
				t.Errorf("imported %d drivers, want %d", len(got.Drivers), tc.wantDrivers)
			}
			if len(got.Documents) != tc.wantDocs {
				t.Errorf("imported %d documents, want %d", len(got.Documents), tc.wantDocs)
			}
			if len(warnings) != len(tc.wantWarnings) {
				t.Fatalf("got %d warnings %v, want %d", len(warnings), warnings, len(tc.wantWarnings))
			}
			for i, want := range tc.wantWarnings {
				if !strings.Contains(warnings[i], want) {
					t.Errorf("warning %d = %q, want it to mention %q", i, warnings[i], want)
				}
			}
			// Imported fleets must always carry a usable policy.
			if got.Blocking == nil || got.Validity == nil {
				t.Error("an imported fleet must come with the default policy in place")
			}
		})
	}
}

func TestReadCSVNormalisesRegistrationNumbers(t *testing.T) {
	in := `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,ka-01-ab-1234,goods,,
document,KA 01 AB 1234,,PUC,2027-10-02
`
	got, warnings, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("the same lorry typed three ways must import cleanly, got %v", warnings)
	}
	if got.Vehicles[0].RegNo != "KA01AB1234" {
		t.Errorf("RegNo = %q, want KA01AB1234", got.Vehicles[0].RegNo)
	}
	if len(got.Documents) != 1 || got.Documents[0].VehicleRegNo != "KA01AB1234" {
		t.Errorf("documents = %+v, want one attached to KA01AB1234", got.Documents)
	}
}

func TestReadCSVLastRowWins(t *testing.T) {
	// A spreadsheet with the same vehicle and the same document kind twice: the
	// later row is the current one, so it replaces the earlier.
	in := `record_type,reg_no,vehicle_type,doc_kind,doc_number,expires_on
vehicle,KA01AB1234,goods,,,
document,KA01AB1234,,Insurance,OLD-POLICY,2027-03-31
document,KA01AB1234,,Insurance,NEW-POLICY,2028-03-31
`
	got, _, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(got.Documents) != 1 {
		t.Fatalf("imported %d documents, want 1 — one current document per kind", len(got.Documents))
	}
	if got.Documents[0].Number != "NEW-POLICY" {
		t.Errorf("Number = %q, want NEW-POLICY", got.Documents[0].Number)
	}
}

// TestReadCSVWithAByteOrderMark covers the file a spreadsheet produces when it
// is told to save as "CSV UTF-8": the first header cell arrives with a byte
// order mark glued to the front of it.
func TestReadCSVWithAByteOrderMark(t *testing.T) {
	in := string(rune(0xFEFF)) + `record_type,reg_no,vehicle_type,doc_kind,expires_on
vehicle,KA01AB1234,goods,,
document,KA01AB1234,,PUC,2027-10-02
`
	got, warnings, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if len(got.Vehicles) != 1 || len(got.Documents) != 1 {
		t.Errorf("imported %d vehicles and %d documents, want 1 and 1", len(got.Vehicles), len(got.Documents))
	}
}

func TestWriteCSVEmptyFleet(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, New()); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if got := strings.TrimRight(buf.String(), "\r\n"); got != strings.Join(CSVHeader, ",") {
		t.Errorf("an empty fleet exports as %q, want just the header row", got)
	}
}
