package fleet

import "fmt"

// Indicative statutory validity periods, in months, each named once and used
// nowhere else.
//
// These are the common defaults under the Motor Vehicles Act, 1988 and the
// Central Motor Vehicles Rules, 1989. They are INDICATIVE ONLY: the real period
// varies by vehicle class, by the age of the vehicle, by permit type and by
// state, and the figures themselves change when the rules are amended. Every
// one of them is overridable per fleet, and the vault never treats a suggested
// date as the date on the paper — the owner types in what the certificate
// actually says.
const (
	// PUCNewVehicleValidityMonths is the first PUC certificate issued with a
	// brand new vehicle.
	PUCNewVehicleValidityMonths = 12
	// PUCValidityMonths is every PUC certificate after the first.
	PUCValidityMonths = 6
	// InsuranceValidityMonths is a standard annual motor policy. Long-term
	// third-party cover sold with a new vehicle runs longer and should be
	// entered from the policy schedule.
	InsuranceValidityMonths = 12
	// FitnessNewTransportValidityMonths is the first certificate of fitness
	// for a new transport vehicle: two years.
	FitnessNewTransportValidityMonths = 24
	// FitnessRenewalValidityMonths is every renewal of the certificate of
	// fitness after the first: one year.
	FitnessRenewalValidityMonths = 12
	// PermitValidityMonths is a goods or passenger permit: five years.
	PermitValidityMonths = 60
)

// ValidityPeriod holds the indicative life of a document in months, split
// because several documents run longer the first time they are issued.
//
// A zero value in either field means the vault has no defensible default and
// will not suggest a date: registration, road tax and driving licence validity
// all depend on the vehicle class, the state and the holder's age by too much
// for a guess to be useful.
type ValidityPeriod struct {
	FirstIssueMonths int `json:"firstIssueMonths"`
	RenewalMonths    int `json:"renewalMonths"`
}

// Known reports whether a default exists for the case asked about.
func (p ValidityPeriod) Known(firstIssue bool) bool { return p.Months(firstIssue) > 0 }

// Months picks the first-issue or the renewal figure.
func (p ValidityPeriod) Months(firstIssue bool) int {
	if firstIssue {
		return p.FirstIssueMonths
	}
	return p.RenewalMonths
}

// DefaultValidity is the indicative table, built from the named constants.
func DefaultValidity() map[DocKind]ValidityPeriod {
	return map[DocKind]ValidityPeriod{
		KindPUC: {
			FirstIssueMonths: PUCNewVehicleValidityMonths,
			RenewalMonths:    PUCValidityMonths,
		},
		KindInsurance: {
			FirstIssueMonths: InsuranceValidityMonths,
			RenewalMonths:    InsuranceValidityMonths,
		},
		KindFitness: {
			FirstIssueMonths: FitnessNewTransportValidityMonths,
			RenewalMonths:    FitnessRenewalValidityMonths,
		},
		KindPermit: {
			FirstIssueMonths: PermitValidityMonths,
			RenewalMonths:    PermitValidityMonths,
		},
		// No default: these vary too much by state, vehicle class and age.
		KindRC:             {},
		KindRoadTax:        {},
		KindDrivingLicence: {},
	}
}

// ValidityNote explains, in one line, what the default for a kind is based on
// and why it must be checked against the paper.
func ValidityNote(kind DocKind) string {
	switch kind {
	case KindPUC:
		return "Indicative: 12 months for the certificate issued with a new vehicle, 6 months after that."
	case KindInsurance:
		return "Indicative: 12 months. Long-term third-party cover sold with a new vehicle runs longer — enter the date on the policy schedule."
	case KindFitness:
		return "Indicative: 2 years for the first certificate of fitness on a new transport vehicle, 1 year on each renewal."
	case KindPermit:
		return "Indicative: 5 years. Permit type and state rules vary."
	case KindRC:
		return "No default. Registration runs 15 years for a private vehicle; for a transport vehicle it is renewed alongside fitness. Enter the date on the RC."
	case KindRoadTax:
		return "No default. Motor vehicle tax is a state levy and may be paid quarterly, annually or as a lifetime tax. Enter the date on the receipt."
	case KindDrivingLicence:
		return "No default. Licence validity depends on the class of licence and the holder's age. Enter the date on the licence."
	}
	return ""
}

// SuggestExpiry proposes an expiry date from an issue date and the indicative
// validity table, so the owner is not doing month arithmetic in their head. It
// is a suggestion to be checked against the certificate, never a substitute for
// it, and it returns false when no defensible default exists for that kind.
//
// Month addition clamps to the end of the target month: a PUC certificate
// issued on 31 August and valid six months runs to 29 February in a leap year
// and 28 February otherwise, not into March.
//
// overrides may be nil, in which case DefaultValidity applies.
func SuggestExpiry(kind DocKind, issuedOn Date, firstIssue bool, overrides map[DocKind]ValidityPeriod) (Date, bool) {
	if issuedOn.IsZero() {
		return Date{}, false
	}
	table := overrides
	if table == nil {
		table = DefaultValidity()
	}
	period, ok := table[kind]
	if !ok {
		period = DefaultValidity()[kind]
	}
	months := period.Months(firstIssue)
	if months <= 0 {
		return Date{}, false
	}
	return issuedOn.AddMonths(months), true
}

// SuggestExpiry proposes an expiry date using this fleet's own validity table.
func (f *Fleet) SuggestExpiry(kind DocKind, issuedOn Date, firstIssue bool) (Date, bool) {
	return SuggestExpiry(kind, issuedOn, firstIssue, f.Validity)
}

// SetValidity overrides the indicative validity period for one kind.
func (f *Fleet) SetValidity(kind DocKind, period ValidityPeriod) error {
	if _, err := ParseDocKind(string(kind)); err != nil {
		return err
	}
	if period.FirstIssueMonths < 0 || period.RenewalMonths < 0 {
		return fmt.Errorf("%s: a validity period cannot be negative", kind.Label())
	}
	if f.Validity == nil {
		f.Validity = DefaultValidity()
	}
	f.Validity[kind] = period
	return nil
}

// SetBlocking changes whether an expired document of this kind stops a vehicle.
func (f *Fleet) SetBlocking(kind DocKind, blocks bool) error {
	if kind.BelongsToDriver() {
		return fmt.Errorf("%s belongs to a driver, not to a vehicle, so it cannot be in the vehicle blocking policy", kind.Label())
	}
	if _, err := ParseDocKind(string(kind)); err != nil {
		return err
	}
	if f.Blocking == nil {
		f.Blocking = DefaultBlocking()
	}
	f.Blocking[kind] = blocks
	return nil
}
