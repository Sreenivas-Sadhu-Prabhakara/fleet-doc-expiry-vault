package fleet

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Fleet is the whole book of records: the vehicles, the drivers, every document
// on file, and the two bits of policy the owner is allowed to change.
type Fleet struct {
	Vehicles  []Vehicle  `json:"vehicles"`
	Drivers   []Driver   `json:"drivers"`
	Documents []Document `json:"documents"`
	// Blocking says which expired document kinds stop a vehicle. Insurance,
	// Fitness, Permit and PUC do by default; RC and road tax are reported but
	// do not, and the owner can change any of them.
	Blocking map[DocKind]bool `json:"blocking"`
	// Validity overrides the indicative statutory validity periods, for when
	// they change or when a vehicle class is treated differently.
	Validity map[DocKind]ValidityPeriod `json:"validity"`
}

// New returns an empty fleet with the default policy in place.
func New() *Fleet {
	return &Fleet{Blocking: DefaultBlocking(), Validity: DefaultValidity()}
}

// Normalise fills in any missing policy and tidies the records, so a data file
// written by an older version, or edited by hand, still loads sensibly.
func (f *Fleet) Normalise() {
	if f.Blocking == nil {
		f.Blocking = DefaultBlocking()
	} else {
		for kind, def := range DefaultBlocking() {
			if _, ok := f.Blocking[kind]; !ok {
				f.Blocking[kind] = def
			}
		}
	}
	if f.Validity == nil {
		f.Validity = DefaultValidity()
	} else {
		for kind, def := range DefaultValidity() {
			if _, ok := f.Validity[kind]; !ok {
				f.Validity[kind] = def
			}
		}
	}
	for i := range f.Vehicles {
		f.Vehicles[i].RegNo = NormaliseRegNo(f.Vehicles[i].RegNo)
	}
	for i := range f.Documents {
		f.Documents[i].VehicleRegNo = NormaliseRegNo(f.Documents[i].VehicleRegNo)
	}
}

// Vehicle finds a vehicle by registration number, however it was typed.
func (f *Fleet) Vehicle(regNo string) (Vehicle, bool) {
	key := NormaliseRegNo(regNo)
	for _, v := range f.Vehicles {
		if v.RegNo == key {
			return v, true
		}
	}
	return Vehicle{}, false
}

// Driver finds a driver by ID.
func (f *Fleet) Driver(id string) (Driver, bool) {
	for _, d := range f.Drivers {
		if d.ID == id {
			return d, true
		}
	}
	return Driver{}, false
}

// VehicleDocuments returns the documents held against one vehicle, in display
// order.
func (f *Fleet) VehicleDocuments(regNo string) []Document {
	key := NormaliseRegNo(regNo)
	var out []Document
	for _, d := range f.Documents {
		if !d.Kind.BelongsToDriver() && d.VehicleRegNo == key {
			out = append(out, d)
		}
	}
	sortDocuments(out)
	return out
}

// DriverDocuments returns the documents held against one driver.
func (f *Fleet) DriverDocuments(driverID string) []Document {
	var out []Document
	for _, d := range f.Documents {
		if d.Kind.BelongsToDriver() && d.DriverID == driverID {
			out = append(out, d)
		}
	}
	sortDocuments(out)
	return out
}

// PutVehicle adds or replaces a vehicle. The registration number is the key, so
// saving the same number twice updates the one record.
func (f *Fleet) PutVehicle(v Vehicle) (Vehicle, error) {
	v.RegNo = NormaliseRegNo(v.RegNo)
	v.Make = strings.TrimSpace(v.Make)
	v.OwnerName = strings.TrimSpace(v.OwnerName)
	if v.RegNo == "" {
		return Vehicle{}, fmt.Errorf("vehicle: a registration number is required")
	}
	if v.Type != TypeGoods && v.Type != TypePassenger {
		return Vehicle{}, fmt.Errorf("vehicle %s: type must be \"goods\" or \"passenger\"", v.RegNo)
	}
	for i, existing := range f.Vehicles {
		if existing.RegNo == v.RegNo {
			f.Vehicles[i] = v
			return v, nil
		}
	}
	f.Vehicles = append(f.Vehicles, v)
	return v, nil
}

// RemoveVehicle drops a vehicle and every document held against it.
func (f *Fleet) RemoveVehicle(regNo string) bool {
	key := NormaliseRegNo(regNo)
	kept := f.Vehicles[:0]
	found := false
	for _, v := range f.Vehicles {
		if v.RegNo == key {
			found = true
			continue
		}
		kept = append(kept, v)
	}
	f.Vehicles = kept
	if !found {
		return false
	}
	docs := f.Documents[:0]
	for _, d := range f.Documents {
		if !d.Kind.BelongsToDriver() && d.VehicleRegNo == key {
			continue
		}
		docs = append(docs, d)
	}
	f.Documents = docs
	return true
}

// PutDriver adds or replaces a driver, generating an ID for a new one.
func (f *Fleet) PutDriver(d Driver) (Driver, error) {
	d.ID = strings.TrimSpace(d.ID)
	d.Name = strings.TrimSpace(d.Name)
	d.Phone = strings.TrimSpace(d.Phone)
	if d.Name == "" {
		return Driver{}, fmt.Errorf("driver: a name is required")
	}
	if d.ID == "" {
		d.ID = f.nextDriverID()
	}
	for i, existing := range f.Drivers {
		if existing.ID == d.ID {
			f.Drivers[i] = d
			return d, nil
		}
	}
	f.Drivers = append(f.Drivers, d)
	return d, nil
}

func (f *Fleet) nextDriverID() string {
	highest := 0
	for _, d := range f.Drivers {
		if n, err := strconv.Atoi(strings.TrimPrefix(d.ID, "D")); err == nil && n > highest {
			highest = n
		}
	}
	return "D" + strconv.Itoa(highest+1)
}

// RemoveDriver drops a driver and their licence record.
func (f *Fleet) RemoveDriver(id string) bool {
	kept := f.Drivers[:0]
	found := false
	for _, d := range f.Drivers {
		if d.ID == id {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	f.Drivers = kept
	if !found {
		return false
	}
	docs := f.Documents[:0]
	for _, d := range f.Documents {
		if d.Kind.BelongsToDriver() && d.DriverID == id {
			continue
		}
		docs = append(docs, d)
	}
	f.Documents = docs
	return true
}

// PutDocument adds or replaces a document. One document of each kind is held
// per vehicle or per driver, so recording a renewed policy replaces the old one
// rather than piling up beside it.
func (f *Fleet) PutDocument(d Document) (Document, error) {
	d.Number = strings.TrimSpace(d.Number)
	d.FileRef = strings.TrimSpace(d.FileRef)
	d.VehicleRegNo = NormaliseRegNo(d.VehicleRegNo)
	d.DriverID = strings.TrimSpace(d.DriverID)
	if _, err := ParseDocKind(string(d.Kind)); err != nil {
		return Document{}, err
	}
	if d.Kind.BelongsToDriver() {
		d.VehicleRegNo = ""
		if _, ok := f.Driver(d.DriverID); !ok {
			return Document{}, fmt.Errorf("driving licence: no driver with ID %q is on file", d.DriverID)
		}
	} else {
		d.DriverID = ""
		if _, ok := f.Vehicle(d.VehicleRegNo); !ok {
			return Document{}, fmt.Errorf("%s: no vehicle with registration %q is on file", d.Kind.Label(), d.VehicleRegNo)
		}
	}
	if d.ExpiresOn.IsZero() {
		return Document{}, fmt.Errorf("%s: an expiry date is required — that is the whole point of the record", d.Kind.Label())
	}
	if !d.IssuedOn.IsZero() && d.ExpiresOn.Before(d.IssuedOn) {
		return Document{}, fmt.Errorf("%s: the expiry date %s falls before the issue date %s", d.Kind.Label(), d.ExpiresOn, d.IssuedOn)
	}
	for i, existing := range f.Documents {
		if existing.Kind == d.Kind && existing.SubjectKey() == d.SubjectKey() {
			f.Documents[i] = d
			return d, nil
		}
	}
	f.Documents = append(f.Documents, d)
	return d, nil
}

// RemoveDocument drops one document by subject and kind.
func (f *Fleet) RemoveDocument(subjectKey string, kind DocKind) bool {
	if !kind.BelongsToDriver() {
		subjectKey = NormaliseRegNo(subjectKey)
	}
	kept := f.Documents[:0]
	found := false
	for _, d := range f.Documents {
		if d.Kind == kind && d.SubjectKey() == subjectKey {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	f.Documents = kept
	return found
}

// kindOrder gives each kind its place in display order.
func kindOrder(k DocKind) int {
	for i, candidate := range AllKinds() {
		if candidate == k {
			return i
		}
	}
	return len(AllKinds())
}

func sortDocuments(docs []Document) {
	sort.SliceStable(docs, func(i, j int) bool {
		return kindOrder(docs[i].Kind) < kindOrder(docs[j].Kind)
	})
}
