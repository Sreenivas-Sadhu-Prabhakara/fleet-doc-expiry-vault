package fleet

import (
	"fmt"
	"strings"
)

// DocKind is one of the statutory documents a transport vehicle or driver must
// be able to produce on demand.
type DocKind string

const (
	// KindRC is the certificate of registration (the "RC book" or RC card).
	KindRC DocKind = "RC"
	// KindInsurance is the motor insurance policy. At minimum, third-party
	// cover is compulsory under section 146 of the Motor Vehicles Act, 1988.
	KindInsurance DocKind = "Insurance"
	// KindFitness is the certificate of fitness (CF) for a transport vehicle,
	// required by section 56 of the Motor Vehicles Act, 1988.
	KindFitness DocKind = "Fitness"
	// KindPermit is the goods or passenger permit required by section 66.
	KindPermit DocKind = "Permit"
	// KindPUC is the pollution under control certificate required by rule 115
	// of the Central Motor Vehicles Rules, 1989.
	KindPUC DocKind = "PUC"
	// KindRoadTax is the motor vehicle tax paid to the state.
	KindRoadTax DocKind = "RoadTax"
	// KindDrivingLicence belongs to a driver, not to a vehicle.
	KindDrivingLicence DocKind = "DrivingLicence"
)

// VehicleKinds are the document kinds held against a vehicle, in the order a
// board or a printout should show them.
func VehicleKinds() []DocKind {
	return []DocKind{KindRC, KindInsurance, KindFitness, KindPermit, KindPUC, KindRoadTax}
}

// DriverKinds are the document kinds held against a driver.
func DriverKinds() []DocKind { return []DocKind{KindDrivingLicence} }

// AllKinds lists every document kind this vault tracks.
func AllKinds() []DocKind {
	return []DocKind{KindRC, KindInsurance, KindFitness, KindPermit, KindPUC, KindRoadTax, KindDrivingLicence}
}

// ParseDocKind reads a kind from input, tolerating case and the common
// shorthands a fleet owner would type ("cf", "fc", "pollution", "tax").
func ParseDocKind(s string) (DocKind, error) {
	key := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(s, "_", " ")), ""))
	switch key {
	case "rc", "registration", "certificateofregistration":
		return KindRC, nil
	case "insurance", "policy", "motorinsurance":
		return KindInsurance, nil
	case "fitness", "cf", "fc", "certificateoffitness":
		return KindFitness, nil
	case "permit":
		return KindPermit, nil
	case "puc", "pucc", "pollution", "emission":
		return KindPUC, nil
	case "roadtax", "tax", "motorvehicletax", "mvtax":
		return KindRoadTax, nil
	case "drivinglicence", "drivinglicense", "dl", "licence", "license":
		return KindDrivingLicence, nil
	}
	return "", fmt.Errorf("document kind: %q is not one of RC, Insurance, Fitness, Permit, PUC, RoadTax, DrivingLicence", s)
}

// Label is the human name for a kind, as it should appear on screen.
func (k DocKind) Label() string {
	switch k {
	case KindRC:
		return "Registration (RC)"
	case KindInsurance:
		return "Insurance"
	case KindFitness:
		return "Fitness (CF)"
	case KindPermit:
		return "Permit"
	case KindPUC:
		return "PUC"
	case KindRoadTax:
		return "Road tax"
	case KindDrivingLicence:
		return "Driving licence"
	}
	return string(k)
}

// BelongsToDriver reports whether this kind is held against a driver rather
// than a vehicle.
func (k DocKind) BelongsToDriver() bool { return k == KindDrivingLicence }

// VehicleType separates the two permit and fitness regimes a small fleet deals
// with.
type VehicleType string

const (
	// TypeGoods is a goods carriage.
	TypeGoods VehicleType = "goods"
	// TypePassenger is a passenger vehicle (bus, maxicab, cab).
	TypePassenger VehicleType = "passenger"
)

// ParseVehicleType reads a vehicle type from input.
func ParseVehicleType(s string) (VehicleType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "goods", "goods carriage", "cargo", "truck", "lcv":
		return TypeGoods, nil
	case "passenger", "passenger vehicle", "bus", "cab", "maxicab":
		return TypePassenger, nil
	}
	return "", fmt.Errorf("vehicle type: %q is not \"goods\" or \"passenger\"", s)
}

// Label is the human name for a vehicle type.
func (t VehicleType) Label() string {
	switch t {
	case TypeGoods:
		return "Goods carriage"
	case TypePassenger:
		return "Passenger vehicle"
	}
	return string(t)
}

// Vehicle is one vehicle in the fleet, keyed by its registration number.
type Vehicle struct {
	RegNo     string      `json:"regNo"`
	Type      VehicleType `json:"type"`
	Make      string      `json:"make"`
	OwnerName string      `json:"ownerName"`
}

// Driver is one driver, keyed by an ID the vault generates. A driving licence
// is held against the driver, because the licence travels with the person and
// not with the vehicle.
type Driver struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Document is one statutory document on file.
//
// A vehicle document carries VehicleRegNo; a driving licence carries DriverID
// instead. FileRef is a path or a note pointing at the scan on the owner's own
// machine — the vault records where the paper is, it never uploads it.
type Document struct {
	VehicleRegNo string  `json:"vehicleRegNo"`
	DriverID     string  `json:"driverId"`
	Kind         DocKind `json:"kind"`
	Number       string  `json:"number"`
	IssuedOn     Date    `json:"issuedOn"`
	ExpiresOn    Date    `json:"expiresOn"`
	FileRef      string  `json:"fileRef"`
}

// SubjectKey identifies what the document belongs to: a registration number for
// a vehicle document, or a driver ID for a licence.
func (d Document) SubjectKey() string {
	if d.Kind.BelongsToDriver() {
		return d.DriverID
	}
	return d.VehicleRegNo
}

// NormaliseRegNo puts a registration number into the one form the vault keys
// on: upper case, no spaces, no hyphens. A fleet owner typing "ka 01 ab 1234",
// "KA-01-AB-1234" and "KA01AB1234" means the same lorry every time.
func NormaliseRegNo(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// PrettyRegNo groups a normalised registration number the way a number plate
// does, e.g. "KA01AB1234" becomes "KA 01 AB 1234". Anything that does not fit
// the familiar pattern is returned unchanged.
func PrettyRegNo(s string) string {
	n := NormaliseRegNo(s)
	var groups []string
	start := 0
	isDigit := func(i int) bool { return n[i] >= '0' && n[i] <= '9' }
	for i := 1; i <= len(n); i++ {
		if i == len(n) || isDigit(i) != isDigit(i-1) {
			groups = append(groups, n[start:i])
			start = i
		}
	}
	if len(groups) < 2 || len(groups) > 4 {
		return n
	}
	return strings.Join(groups, " ")
}
