package fleet

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CSVHeader is the one row format used for both export and import. A single
// table keeps the file openable in any spreadsheet: record_type says what each
// row is, and the columns that do not apply to that row are left empty.
var CSVHeader = []string{
	"record_type",
	"reg_no",
	"vehicle_type",
	"make",
	"owner_name",
	"driver_id",
	"driver_name",
	"driver_phone",
	"doc_kind",
	"doc_number",
	"issued_on",
	"expires_on",
	"file_ref",
}

const (
	rowVehicle  = "vehicle"
	rowDriver   = "driver"
	rowDocument = "document"
)

// WriteCSV writes the whole fleet as one table: the vehicles, then the drivers,
// then every document. The file round-trips through ReadCSV without loss.
func WriteCSV(w io.Writer, f *Fleet) error {
	out := csv.NewWriter(w)
	if err := out.Write(CSVHeader); err != nil {
		return err
	}
	for _, v := range f.Vehicles {
		if err := out.Write([]string{rowVehicle, v.RegNo, string(v.Type), v.Make, v.OwnerName, "", "", "", "", "", "", "", ""}); err != nil {
			return err
		}
	}
	for _, d := range f.Drivers {
		if err := out.Write([]string{rowDriver, "", "", "", "", d.ID, d.Name, d.Phone, "", "", "", "", ""}); err != nil {
			return err
		}
	}
	for _, doc := range f.Documents {
		if err := out.Write([]string{
			rowDocument,
			doc.VehicleRegNo, "", "", "",
			doc.DriverID, "", "",
			string(doc.Kind), doc.Number,
			doc.IssuedOn.String(), doc.ExpiresOn.String(), doc.FileRef,
		}); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// ReadCSV reads a fleet back out of the table.
//
// It is deliberately forgiving, because the file may have been edited in a
// spreadsheet: the header may be in any column order, header names may differ
// in case or use spaces instead of underscores, blank rows are skipped, and a
// row the vault cannot use is reported as a warning rather than failing the
// whole import. Vehicles and drivers are read first so that documents can be
// attached to them whatever order the rows appear in.
func ReadCSV(r io.Reader) (*Fleet, []string, error) {
	in := csv.NewReader(r)
	in.FieldsPerRecord = -1
	in.TrimLeadingSpace = true
	records, err := in.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("the CSV is empty")
	}

	index := map[string]int{}
	for i, name := range records[0] {
		index[csvKey(name)] = i
	}
	if _, ok := index["record_type"]; !ok {
		return nil, nil, fmt.Errorf("the CSV has no record_type column — the first row must be the header: %s", strings.Join(CSVHeader, ","))
	}

	get := func(row []string, name string) string {
		i, ok := index[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	f := New()
	var warnings []string
	warn := func(line int, format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf("row %d: %s", line, fmt.Sprintf(format, args...)))
	}

	// Two passes: subjects first, then the documents that hang off them.
	for pass := 0; pass < 2; pass++ {
		for i, row := range records {
			if i == 0 || isBlankRow(row) {
				continue
			}
			line := i + 1
			kind := strings.ToLower(get(row, "record_type"))
			isDoc := kind == rowDocument
			if (pass == 0) == isDoc {
				continue
			}
			switch kind {
			case rowVehicle:
				vehicleType, err := ParseVehicleType(get(row, "vehicle_type"))
				if err != nil {
					warn(line, "%v", err)
					continue
				}
				if _, err := f.PutVehicle(Vehicle{
					RegNo:     get(row, "reg_no"),
					Type:      vehicleType,
					Make:      get(row, "make"),
					OwnerName: get(row, "owner_name"),
				}); err != nil {
					warn(line, "%v", err)
				}
			case rowDriver:
				if _, err := f.PutDriver(Driver{
					ID:    get(row, "driver_id"),
					Name:  get(row, "driver_name"),
					Phone: get(row, "driver_phone"),
				}); err != nil {
					warn(line, "%v", err)
				}
			case rowDocument:
				docKind, err := ParseDocKind(get(row, "doc_kind"))
				if err != nil {
					warn(line, "%v", err)
					continue
				}
				doc := Document{
					VehicleRegNo: get(row, "reg_no"),
					DriverID:     get(row, "driver_id"),
					Kind:         docKind,
					Number:       get(row, "doc_number"),
					FileRef:      get(row, "file_ref"),
				}
				if raw := get(row, "issued_on"); raw != "" {
					if doc.IssuedOn, err = ParseDate(raw); err != nil {
						warn(line, "issue date: %v", err)
						continue
					}
				}
				raw := get(row, "expires_on")
				if raw == "" {
					warn(line, "%s for %q has no expiry date — skipped", docKind.Label(), doc.SubjectKey())
					continue
				}
				if doc.ExpiresOn, err = ParseDate(raw); err != nil {
					warn(line, "expiry date: %v", err)
					continue
				}
				if _, err := f.PutDocument(doc); err != nil {
					warn(line, "%v", err)
				}
			default:
				warn(line, "record_type %q is not vehicle, driver or document — skipped", get(row, "record_type"))
			}
		}
	}
	return f, warnings, nil
}

// csvKey normalises a header cell so "Reg No", "reg-no" and "reg_no" all match.
func csvKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.NewReplacer(" ", "_", "-", "_", ".", "").Replace(name)
	// A spreadsheet that saves as "CSV UTF-8" puts a byte order mark in front
	// of the very first header cell. Strip it, or the record_type column goes
	// missing and the whole import is refused.
	return strings.TrimPrefix(name, string(rune(0xFEFF)))
}

func isBlankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
