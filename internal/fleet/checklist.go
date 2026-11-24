package fleet

import "fmt"

// Checklist is what to take to the renewal, printable as it stands.
type Checklist struct {
	Kind  DocKind  `json:"kind"`
	Label string   `json:"label"`
	Where string   `json:"where"`
	Items []string `json:"items"`
	Forms []string `json:"forms"`
	Note  string   `json:"note"`
}

// renewalNote is appended to every checklist. The lists below are what a small
// fleet is usually asked for; the authority is the authority.
const renewalNote = "Indicative list. The exact papers, forms and fees vary by state, by vehicle class and by permit type, and they change. Confirm the current requirement with your RTO or transport department office before you travel."

// RenewalChecklist returns the documents typically needed to renew one kind of
// document, so the owner makes one trip instead of three.
func RenewalChecklist(kind DocKind) (Checklist, error) {
	if _, err := ParseDocKind(string(kind)); err != nil {
		return Checklist{}, err
	}
	c := Checklist{Kind: kind, Label: kind.Label(), Note: renewalNote}
	switch kind {
	case KindInsurance:
		c.Where = "Your insurer or an authorised intermediary. No RTO visit is needed."
		c.Items = []string{
			"The expiring or expired policy document, or the cover note",
			"Registration certificate (RC) — number, chassis number and engine number",
			"A valid PUC certificate (insurers ask for one before issuing cover)",
			"Previous claim history and no-claim bonus details, if you are changing insurer",
			"Owner identity and address proof",
			"Vehicle inspection by the insurer, if the policy has already lapsed",
		}
	case KindFitness:
		c.Where = "RTO or an authorised automated testing station, with the vehicle itself."
		c.Forms = []string{"CMVR Form 25 — application for renewal of a certificate of fitness"}
		c.Items = []string{
			"The vehicle, presented for inspection",
			"Registration certificate (RC)",
			"Valid insurance policy",
			"Valid PUC certificate",
			"Road tax paid up to date, with the receipt",
			"The current permit, for a goods or passenger vehicle",
			"The expiring certificate of fitness",
			"Fitness and inspection fee as notified by the state",
		}
	case KindPermit:
		c.Where = "The transport authority that issued the permit, or the RTO."
		c.Forms = []string{"The prescribed application form for your permit type (for example Form 46 and Form 48 for a national permit for goods carriages) — the RTO will confirm which applies"}
		c.Items = []string{
			"The expiring permit",
			"Registration certificate (RC)",
			"Valid certificate of fitness",
			"Valid insurance policy",
			"Valid PUC certificate",
			"Tax clearance or the latest road tax receipt",
			"Permit fee and any authorisation fee",
		}
	case KindPUC:
		c.Where = "Any authorised PUC testing centre. The certificate is issued on the spot once the vehicle passes."
		c.Items = []string{
			"The vehicle, brought to the centre",
			"Registration number (the centre reads it from the plate and the RC)",
			"The previous PUC certificate, if you have it",
			"Test fee, which is fixed by the state",
		}
	case KindRC:
		c.Where = "The RTO where the vehicle is registered."
		c.Forms = []string{"CMVR Form 25 — application for renewal of a certificate of registration"}
		c.Items = []string{
			"The original registration certificate",
			"Valid insurance policy",
			"Valid PUC certificate",
			"Valid certificate of fitness, for a transport vehicle",
			"Road tax paid up to date, with the receipt",
			"Chassis number imprint (pencil print)",
			"Owner identity and address proof",
			"Renewal fee, plus green tax where the state levies it",
		}
	case KindRoadTax:
		c.Where = "The state transport department — counter or state portal."
		c.Items = []string{
			"Registration certificate (RC)",
			"The previous tax payment receipt",
			"Valid insurance policy",
			"Purchase invoice and chassis details, for a vehicle being taxed the first time",
			"The tax amount for your vehicle class and the period being paid",
		}
	case KindDrivingLicence:
		c.Where = "The RTO that holds the licence, or the licensing authority where the driver now lives."
		c.Forms = []string{
			"CMVR Form 9 — application for renewal of a driving licence",
			"CMVR Form 1 — self-declaration of physical fitness",
			"CMVR Form 1A — medical certificate from a registered practitioner, required for a transport licence and for a driver aged 40 or above",
		}
		c.Items = []string{
			"The expiring driving licence",
			"Proof of address",
			"Passport-size photographs",
			"Renewal fee, plus a late fee if the licence expired more than 30 days ago",
			"A fresh driving test, if the licence has been expired long enough for the authority to require one",
		}
	default:
		return Checklist{}, fmt.Errorf("no renewal checklist for %q", kind)
	}
	return c, nil
}

// AllChecklists returns every checklist in display order, for the printable
// sheet.
func AllChecklists() []Checklist {
	out := make([]Checklist, 0, len(AllKinds()))
	for _, kind := range AllKinds() {
		if c, err := RenewalChecklist(kind); err == nil {
			out = append(out, c)
		}
	}
	return out
}
