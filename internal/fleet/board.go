package fleet

import "sort"

// SubjectKind says whether a board row belongs to a vehicle or to a driver.
type SubjectKind string

const (
	// SubjectVehicle is a document held against a vehicle.
	SubjectVehicle SubjectKind = "vehicle"
	// SubjectDriver is a document held against a driver.
	SubjectDriver SubjectKind = "driver"
)

// BoardRow is one document on the fleet board, with everything a row needs so
// the caller never has to look anything up again.
type BoardRow struct {
	SubjectKind  SubjectKind `json:"subjectKind"`
	SubjectKey   string      `json:"subjectKey"`
	SubjectLabel string      `json:"subjectLabel"`
	Detail       string      `json:"detail"`
	Kind         DocKind     `json:"kind"`
	Label        string      `json:"label"`
	Number       string      `json:"number"`
	IssuedOn     Date        `json:"issuedOn"`
	ExpiresOn    Date        `json:"expiresOn"`
	FileRef      string      `json:"fileRef"`
	Status       DocStatus   `json:"status"`
	Note         string      `json:"note"`
}

// FleetBoard lists every document on file across the whole fleet — vehicle
// documents and driver licences alike — sorted by urgency: the expired ones
// first, longest overdue at the top, then the soonest to expire. Documents with
// no expiry date recorded sort last, because nothing can be said about them.
func (f *Fleet) FleetBoard(asOf Date) []BoardRow {
	rows := make([]BoardRow, 0, len(f.Documents))
	for _, doc := range f.Documents {
		rows = append(rows, f.row(doc, asOf))
	}
	SortByUrgency(rows)
	return rows
}

// LicenceBoard lists the driving licences on file, sorted by urgency. A licence
// is attached to a driver, so an expired one grounds the person and not one
// particular vehicle.
func (f *Fleet) LicenceBoard(asOf Date) []BoardRow {
	rows := make([]BoardRow, 0, len(f.Drivers))
	for _, doc := range f.Documents {
		if doc.Kind.BelongsToDriver() {
			rows = append(rows, f.row(doc, asOf))
		}
	}
	SortByUrgency(rows)
	return rows
}

// VehicleBoard lists the documents of one vehicle, sorted by urgency.
func (f *Fleet) VehicleBoard(regNo string, asOf Date) []BoardRow {
	docs := f.VehicleDocuments(regNo)
	rows := make([]BoardRow, 0, len(docs))
	for _, doc := range docs {
		rows = append(rows, f.row(doc, asOf))
	}
	SortByUrgency(rows)
	return rows
}

func (f *Fleet) row(doc Document, asOf Date) BoardRow {
	row := BoardRow{
		SubjectKey: doc.SubjectKey(),
		Kind:       doc.Kind,
		Label:      doc.Kind.Label(),
		Number:     doc.Number,
		IssuedOn:   doc.IssuedOn,
		ExpiresOn:  doc.ExpiresOn,
		FileRef:    doc.FileRef,
		Status:     Status(doc.ExpiresOn, asOf),
	}
	if doc.Kind.BelongsToDriver() {
		row.SubjectKind = SubjectDriver
		row.SubjectLabel = doc.DriverID
		if driver, ok := f.Driver(doc.DriverID); ok {
			row.SubjectLabel = driver.Name
			row.Detail = driver.Phone
		}
		row.Note = "Driver document"
		return row
	}
	row.SubjectKind = SubjectVehicle
	row.SubjectLabel = PrettyRegNo(doc.VehicleRegNo)
	if vehicle, ok := f.Vehicle(doc.VehicleRegNo); ok {
		row.Detail = vehicle.Make
		if row.Detail == "" {
			row.Detail = vehicle.Type.Label()
		}
	}
	if f.Blocking[doc.Kind] {
		row.Note = "Stops the vehicle when expired"
	}
	return row
}

// SortByUrgency orders rows the way a board should read: expired first, longest
// overdue at the top, then the soonest expiry, with undated rows last. Ties
// break on subject then document kind so the board never shuffles between two
// identical refreshes.
func SortByUrgency(rows []BoardRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ExpiresOn.IsZero() != b.ExpiresOn.IsZero() {
			return b.ExpiresOn.IsZero()
		}
		if !a.ExpiresOn.IsZero() && a.ExpiresOn != b.ExpiresOn {
			return a.ExpiresOn.Before(b.ExpiresOn)
		}
		if a.SubjectLabel != b.SubjectLabel {
			return a.SubjectLabel < b.SubjectLabel
		}
		return kindOrder(a.Kind) < kindOrder(b.Kind)
	})
}

// Counts summarises a board for the headline strip: how many documents sit in
// each traffic-light band.
type Counts struct {
	Total   int `json:"total"`
	Expired int `json:"expired"`
	DueSoon int `json:"dueSoon"`
	Valid   int `json:"valid"`
	Unknown int `json:"unknown"`
}

// Summarise counts the rows by status.
func Summarise(rows []BoardRow) Counts {
	c := Counts{Total: len(rows)}
	for _, row := range rows {
		switch row.Status.Kind {
		case StatusExpired:
			c.Expired++
		case StatusDueSoon:
			c.DueSoon++
		case StatusValid:
			c.Valid++
		default:
			c.Unknown++
		}
	}
	return c
}

// NextDue returns the single most pressing expiry among the documents given:
// the earliest expiry date on file. An already-expired document is the soonest
// of all, and is returned as such — that is the one the owner must deal with
// first. Ties break on document kind, so the answer never changes between two
// calls on the same records. The second return value is false when none of the
// documents carries an expiry date.
func NextDue(docs []Document) (Document, bool) {
	var best Document
	found := false
	for _, doc := range docs {
		if doc.ExpiresOn.IsZero() {
			continue
		}
		switch {
		case !found,
			doc.ExpiresOn.Before(best.ExpiresOn),
			doc.ExpiresOn == best.ExpiresOn && kindOrder(doc.Kind) < kindOrder(best.Kind):
			best, found = doc, true
		}
	}
	return best, found
}

// NextDue returns the most pressing expiry for one vehicle in the fleet.
func (f *Fleet) NextDue(regNo string) (Document, bool) {
	return NextDue(f.VehicleDocuments(regNo))
}
