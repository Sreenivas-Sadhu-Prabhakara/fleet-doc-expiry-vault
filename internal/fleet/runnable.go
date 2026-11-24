package fleet

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultBlocking is the statutory default: an expired insurance policy,
// certificate of fitness, permit or PUC certificate makes the vehicle illegal
// to run, and running it anyway voids the cover.
//
// Registration and road tax are recorded and reported, but by default they do
// not stop the vehicle — an expired RC or an unpaid tax demand is a penalty and
// a renewal job, not a roadside immobilisation. Either can be switched to
// blocking, and any of the four defaults can be switched off for a vehicle
// class that genuinely does not need that document (a permit-exempt carriage
// under section 66 of the Motor Vehicles Act, 1988, for instance).
func DefaultBlocking() map[DocKind]bool {
	return map[DocKind]bool{
		KindInsurance: true,
		KindFitness:   true,
		KindPermit:    true,
		KindPUC:       true,
		KindRC:        false,
		KindRoadTax:   false,
	}
}

// Blocker is one document standing in the way, with enough detail to act on it.
type Blocker struct {
	Kind      DocKind   `json:"kind"`
	Label     string    `json:"label"`
	Number    string    `json:"number"`
	ExpiresOn Date      `json:"expiresOn"`
	FileRef   string    `json:"fileRef"`
	Status    DocStatus `json:"status"`
	Reason    string    `json:"reason"`
}

// RunVerdict answers the one question a fleet owner needs answered before the
// vehicle leaves the yard.
type RunVerdict struct {
	RegNo  string `json:"regNo"`
	AsOf   Date   `json:"asOf"`
	CanRun bool   `json:"canRun"`
	// Blocking lists the expired documents that stop the vehicle, named.
	Blocking []Blocker `json:"blocking"`
	// Missing lists blocking kinds with nothing on file at all, which is just
	// as unprovable at a checkpoint as an expired one.
	Missing []Blocker `json:"missing"`
	// Reported lists expired documents that are set not to block.
	Reported []Blocker `json:"reported"`
	// Expiring lists documents inside a reminder tier but still valid today.
	Expiring []Blocker `json:"expiring"`
	Headline string    `json:"headline"`
	Detail   string    `json:"detail"`
}

// CanRunToday decides whether one vehicle may legally be driven on a given day,
// and when the answer is no, names exactly which documents are in the way.
//
// blocking may be nil, in which case DefaultBlocking applies.
func CanRunToday(v Vehicle, docs []Document, asOf Date, blocking map[DocKind]bool) RunVerdict {
	if blocking == nil {
		blocking = DefaultBlocking()
	}
	verdict := RunVerdict{RegNo: v.RegNo, AsOf: asOf, CanRun: true}

	onFile := make(map[DocKind]Document, len(docs))
	for _, d := range docs {
		if d.Kind.BelongsToDriver() {
			continue // a licence belongs to the driver, not to the vehicle
		}
		onFile[d.Kind] = d
	}

	for _, kind := range VehicleKinds() {
		doc, held := onFile[kind]
		blocks := blocking[kind]
		if !held {
			if blocks {
				verdict.Missing = append(verdict.Missing, Blocker{
					Kind:   kind,
					Label:  kind.Label(),
					Status: DocStatus{Kind: StatusUnknown, Band: BandGrey},
					Reason: fmt.Sprintf("No %s on file — record it, or switch it off in the policy if this vehicle does not need one.", kind.Label()),
				})
			}
			continue
		}
		status := Status(doc.ExpiresOn, asOf)
		item := Blocker{
			Kind:      kind,
			Label:     kind.Label(),
			Number:    doc.Number,
			ExpiresOn: doc.ExpiresOn,
			FileRef:   doc.FileRef,
			Status:    status,
		}
		switch {
		case status.Expired() && blocks:
			item.Reason = fmt.Sprintf("%s expired on %s, %s ago.", kind.Label(), doc.ExpiresOn.Pretty(), plural(status.DaysOverdue, "day", "days"))
			verdict.Blocking = append(verdict.Blocking, item)
		case status.Expired():
			item.Reason = fmt.Sprintf("%s expired on %s — recorded, but set not to stop the vehicle.", kind.Label(), doc.ExpiresOn.Pretty())
			verdict.Reported = append(verdict.Reported, item)
		case status.Kind == StatusDueSoon:
			item.Reason = fmt.Sprintf("%s expires on %s — %s.", kind.Label(), doc.ExpiresOn.Pretty(), strings.ToLower(status.Describe()))
			verdict.Expiring = append(verdict.Expiring, item)
		case status.Kind == StatusUnknown && blocks:
			item.Reason = fmt.Sprintf("%s has no expiry date on file.", kind.Label())
			verdict.Missing = append(verdict.Missing, item)
		}
	}

	sortBlockers(verdict.Blocking)
	sortBlockers(verdict.Missing)
	sortBlockers(verdict.Reported)
	sortBlockers(verdict.Expiring)

	verdict.CanRun = len(verdict.Blocking) == 0 && len(verdict.Missing) == 0
	verdict.Headline, verdict.Detail = runHeadline(verdict)
	return verdict
}

func runHeadline(v RunVerdict) (headline, detail string) {
	plate := PrettyRegNo(v.RegNo)
	if v.CanRun {
		headline = plate + " can run today"
		switch {
		case len(v.Expiring) > 0:
			detail = fmt.Sprintf("Every blocking document is current. %s coming up for renewal: %s.",
				plural(len(v.Expiring), "document", "documents"), joinLabels(v.Expiring))
		case len(v.Reported) > 0:
			detail = fmt.Sprintf("Every blocking document is current, but %s expired and set not to block: %s.",
				plural(len(v.Reported), "document is", "documents are"), joinLabels(v.Reported))
		default:
			detail = "Every document on file is current."
		}
		return headline, detail
	}
	headline = plate + " must not run today"
	var parts []string
	if len(v.Blocking) > 0 {
		parts = append(parts, "Expired: "+joinLabels(v.Blocking))
	}
	if len(v.Missing) > 0 {
		parts = append(parts, "Not on file: "+joinLabels(v.Missing))
	}
	detail = strings.Join(parts, ". ") + "."
	return headline, detail
}

func joinLabels(items []Blocker) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Label)
	}
	switch len(names) {
	case 0:
		return "none"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func sortBlockers(items []Blocker) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Status.DaysOverdue != items[j].Status.DaysOverdue {
			return items[i].Status.DaysOverdue > items[j].Status.DaysOverdue
		}
		return kindOrder(items[i].Kind) < kindOrder(items[j].Kind)
	})
}

// CanRunToday answers the question for one vehicle in the fleet, using the
// fleet's own blocking policy.
func (f *Fleet) CanRunToday(regNo string, asOf Date) (RunVerdict, bool) {
	v, ok := f.Vehicle(regNo)
	if !ok {
		return RunVerdict{}, false
	}
	return CanRunToday(v, f.VehicleDocuments(v.RegNo), asOf, f.Blocking), true
}
