package fleet

import "fmt"

// Reminder tiers, in days before expiry. These are the three moments a fleet
// owner can still act: a month out there is time to book a fitness slot, a
// fortnight out the insurer can still be paid without a break in cover, three
// days out it is a scramble.
const (
	// TierNoticeDays is the first, calm reminder.
	TierNoticeDays = 30
	// TierWarnDays is the second reminder: book the slot now.
	TierWarnDays = 15
	// TierUrgentDays is the last reminder before the vehicle is off the road.
	TierUrgentDays = 3
)

// StatusKind is the state of one document on a given day.
type StatusKind string

const (
	// StatusValid means more than TierNoticeDays left.
	StatusValid StatusKind = "valid"
	// StatusDueSoon means within TierNoticeDays, but not yet expired.
	StatusDueSoon StatusKind = "dueSoon"
	// StatusExpired means the expiry date has passed.
	StatusExpired StatusKind = "expired"
	// StatusUnknown means no expiry date has been recorded.
	StatusUnknown StatusKind = "unknown"
)

// Traffic-light bands, so every surface colours a document the same way.
const (
	BandGreen = "green"
	BandAmber = "amber"
	BandRed   = "red"
	BandGrey  = "grey"
)

// DocStatus is the verdict on one document on one day.
type DocStatus struct {
	Kind StatusKind `json:"kind"`
	// DaysLeft is the whole days from asOf to the expiry date. Zero on the
	// expiry day itself, and zero when expired or unknown.
	DaysLeft int `json:"daysLeft"`
	// DaysOverdue is the whole days since the expiry date passed, and is at
	// least 1 whenever Kind is StatusExpired.
	DaysOverdue int `json:"daysOverdue"`
	// Tier is the reminder tier that fired: 30, 15 or 3 when due soon, and 0
	// otherwise.
	Tier int `json:"tier"`
	// Band is the traffic light: green, amber, red or grey.
	Band string `json:"band"`
}

// Expired reports whether the document has run out.
func (s DocStatus) Expired() bool { return s.Kind == StatusExpired }

// Status grades one expiry date against the day being asked about.
//
// A document is treated as valid through the whole of its expiry date, which is
// how an expiry date is printed on an Indian insurance policy or PUC
// certificate: "valid upto 31-03-2027" includes 31 March. So the expiry day
// itself is the most urgent due-soon tier (zero days left), and the day after
// it is expired by one day.
func Status(expiresOn, asOf Date) DocStatus {
	if expiresOn.IsZero() {
		return DocStatus{Kind: StatusUnknown, Band: BandGrey}
	}
	left := DaysBetween(asOf, expiresOn)
	switch {
	case left < 0:
		return DocStatus{Kind: StatusExpired, DaysOverdue: -left, Band: BandRed}
	case left <= TierUrgentDays:
		return DocStatus{Kind: StatusDueSoon, DaysLeft: left, Tier: TierUrgentDays, Band: BandAmber}
	case left <= TierWarnDays:
		return DocStatus{Kind: StatusDueSoon, DaysLeft: left, Tier: TierWarnDays, Band: BandAmber}
	case left <= TierNoticeDays:
		return DocStatus{Kind: StatusDueSoon, DaysLeft: left, Tier: TierNoticeDays, Band: BandAmber}
	default:
		return DocStatus{Kind: StatusValid, DaysLeft: left, Band: BandGreen}
	}
}

// Describe puts the status into the one line a board row needs.
func (s DocStatus) Describe() string {
	switch s.Kind {
	case StatusExpired:
		return fmt.Sprintf("Expired %s ago", plural(s.DaysOverdue, "day", "days"))
	case StatusDueSoon:
		if s.DaysLeft == 0 {
			return "Expires today"
		}
		return fmt.Sprintf("%s left", plural(s.DaysLeft, "day", "days"))
	case StatusValid:
		return fmt.Sprintf("%s left", plural(s.DaysLeft, "day", "days"))
	}
	return "No expiry date on file"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
