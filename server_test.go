package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/fleet-doc-expiry-vault/internal/fleet"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "vault.json"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	return NewServer(store, ui)
}

func do(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, request)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decoding %s: %v", recorder.Body.String(), err)
	}
}

// seed puts one lorry with a full set of documents on file, with insurance
// already expired, and returns the server.
func seed(t *testing.T, s *Server) {
	t.Helper()
	if got := do(t, s, "PUT", "/api/vehicle", `{"regNo":"KA 01 AB 1234","type":"goods","make":"Tata 407","ownerName":"Ravi"}`).Code; got != http.StatusOK {
		t.Fatalf("PUT /api/vehicle = %d", got)
	}
	docs := []string{
		`{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"Insurance","number":"POL-9","issuedOn":"2025-04-01","expiresOn":"2026-03-31","fileRef":"scans/ins.pdf"}`,
		`{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"Fitness","number":"CF-3","issuedOn":"","expiresOn":"2030-02-01","fileRef":""}`,
		`{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"Permit","number":"PRM-7","issuedOn":"","expiresOn":"2031-06-01","fileRef":""}`,
		`{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"PUC","number":"PUC-2","issuedOn":"","expiresOn":"2030-08-01","fileRef":""}`,
	}
	for _, doc := range docs {
		if got := do(t, s, "PUT", "/api/document", doc); got.Code != http.StatusOK {
			t.Fatalf("PUT /api/document %s = %d: %s", doc, got.Code, got.Body.String())
		}
	}
}

func TestServerServesTheEmbeddedUI(t *testing.T) {
	s := testServer(t)
	recorder := do(t, s, "GET", "/", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET / = %d", recorder.Code)
	}
	page := recorder.Body.String()
	for _, want := range []string{
		"Vehicle document expiry tracker for small transport fleets",
		"assets/app.js",
		"Content-Security-Policy",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the served page does not contain %q", want)
		}
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("responses must set X-Content-Type-Options")
	}

	for _, asset := range []string{"/assets/style.css", "/assets/accent.css", "/assets/app.css", "/assets/app.js", "/assets/theme-init.js", "/assets/favicon.svg"} {
		if got := do(t, s, "GET", asset, "").Code; got != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", asset, got)
		}
	}
}

func TestServerStateAndBoard(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	var state stateResponse
	decode(t, do(t, s, "GET", "/api/state", ""), &state)
	if len(state.Vehicles) != 1 || state.Vehicles[0].RegNo != "KA01AB1234" {
		t.Errorf("vehicles = %+v, want the normalised KA01AB1234", state.Vehicles)
	}
	if len(state.Documents) != 4 {
		t.Errorf("documents = %d, want 4", len(state.Documents))
	}
	if len(state.Kinds) != len(fleet.AllKinds()) {
		t.Errorf("kinds = %d, want %d", len(state.Kinds), len(fleet.AllKinds()))
	}
	if got := state.Tiers; len(got) != 3 || got[0] != 30 || got[1] != 15 || got[2] != 3 {
		t.Errorf("tiers = %v, want [30 15 3]", got)
	}
	if state.DataFile == "" || !filepath.IsAbs(state.DataFile) {
		t.Errorf("dataFile = %q, want the absolute path so the owner knows where their records are", state.DataFile)
	}

	var board boardResponse
	decode(t, do(t, s, "GET", "/api/board?asOf=2027-03-01", ""), &board)
	if len(board.Rows) != 4 {
		t.Fatalf("board rows = %d, want 4", len(board.Rows))
	}
	if board.Rows[0].Kind != fleet.KindInsurance || board.Rows[0].Status.Kind != fleet.StatusExpired {
		t.Errorf("first row = %s / %s, want the expired insurance at the top", board.Rows[0].Kind, board.Rows[0].Status.Kind)
	}
	if board.Counts.Expired != 1 || board.Counts.Valid != 3 {
		t.Errorf("counts = %+v, want 1 expired and 3 valid", board.Counts)
	}
	if board.AsOf != fleet.NewDate(2027, time.March, 1) {
		t.Errorf("asOf = %s, want the date that was asked for", board.AsOf)
	}
}

func TestServerCanRun(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	var answer canRunResponse
	decode(t, do(t, s, "GET", "/api/can-run?regNo=ka+01+ab+1234&asOf=2027-03-01", ""), &answer)
	if answer.Verdict.CanRun {
		t.Fatalf("an expired insurance policy must ground the lorry, got %+v", answer.Verdict)
	}
	if len(answer.Verdict.Blocking) != 1 || answer.Verdict.Blocking[0].Kind != fleet.KindInsurance {
		t.Errorf("blocking = %+v, want only the insurance", answer.Verdict.Blocking)
	}
	if !strings.Contains(answer.Verdict.Headline, "must not run today") {
		t.Errorf("headline = %q", answer.Verdict.Headline)
	}
	if answer.NextDue == nil || answer.NextDue.Kind != fleet.KindInsurance {
		t.Errorf("nextDue = %+v, want the insurance", answer.NextDue)
	}
	if len(answer.Rows) != 4 {
		t.Errorf("rows = %d, want every document on file", len(answer.Rows))
	}

	// Renew the policy, and the same vehicle may run.
	if got := do(t, s, "PUT", "/api/document", `{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"Insurance","number":"POL-10","issuedOn":"2026-04-01","expiresOn":"2028-03-31","fileRef":""}`); got.Code != http.StatusOK {
		t.Fatalf("renewing the policy = %d: %s", got.Code, got.Body.String())
	}
	decode(t, do(t, s, "GET", "/api/can-run?regNo=KA01AB1234&asOf=2027-03-01", ""), &answer)
	if !answer.Verdict.CanRun {
		t.Errorf("after renewal the lorry must run, got %+v", answer.Verdict)
	}

	if got := do(t, s, "GET", "/api/can-run?regNo=KA99ZZ0000", "").Code; got != http.StatusNotFound {
		t.Errorf("an unknown vehicle = %d, want 404", got)
	}
	if got := do(t, s, "GET", "/api/can-run", "").Code; got != http.StatusBadRequest {
		t.Errorf("no registration number = %d, want 400", got)
	}
	if got := do(t, s, "GET", "/api/board?asOf=whenever", "").Code; got != http.StatusBadRequest {
		t.Errorf("a bad asOf = %d, want 400", got)
	}
}

func TestServerRejectsBadDocuments(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"no expiry date", `{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"PUC","number":"","issuedOn":"","expiresOn":"","fileRef":""}`, http.StatusBadRequest},
		{"unknown vehicle", `{"vehicleRegNo":"KA99ZZ0000","driverId":"","kind":"PUC","number":"","issuedOn":"","expiresOn":"2030-01-01","fileRef":""}`, http.StatusBadRequest},
		{"unknown kind", `{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"TollPass","number":"","issuedOn":"","expiresOn":"2030-01-01","fileRef":""}`, http.StatusBadRequest},
		{"expiry before issue", `{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"PUC","number":"","issuedOn":"2030-01-01","expiresOn":"2029-01-01","fileRef":""}`, http.StatusBadRequest},
		{"a licence with no driver", `{"vehicleRegNo":"","driverId":"D9","kind":"DrivingLicence","number":"","issuedOn":"","expiresOn":"2040-01-01","fileRef":""}`, http.StatusBadRequest},
		{"not JSON at all", `nonsense`, http.StatusBadRequest},
		{"a field the vault does not know", `{"vehicleRegNo":"KA01AB1234","kind":"PUC","expiresOn":"2030-01-01","colour":"red"}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := do(t, s, "PUT", "/api/document", tc.body)
			if recorder.Code != tc.want {
				t.Fatalf("PUT /api/document = %d, want %d: %s", recorder.Code, tc.want, recorder.Body.String())
			}
			var failure struct{ Error string }
			decode(t, recorder, &failure)
			if failure.Error == "" {
				t.Error("a rejection must say why")
			}
		})
	}
}

func TestServerDriversAndLicences(t *testing.T) {
	s := testServer(t)

	recorder := do(t, s, "PUT", "/api/driver", `{"id":"","name":"Suresh Kumar","phone":"9876500000"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT /api/driver = %d: %s", recorder.Code, recorder.Body.String())
	}
	var driver fleet.Driver
	decode(t, recorder, &driver)
	if driver.ID == "" {
		t.Fatal("the vault must generate a driver ID")
	}

	licence := `{"vehicleRegNo":"","driverId":"` + driver.ID + `","kind":"DrivingLicence","number":"KA0120110012345","issuedOn":"2021-05-10","expiresOn":"2027-03-10","fileRef":"scans/dl.pdf"}`
	if got := do(t, s, "PUT", "/api/document", licence); got.Code != http.StatusOK {
		t.Fatalf("PUT licence = %d: %s", got.Code, got.Body.String())
	}

	var licences boardResponse
	decode(t, do(t, s, "GET", "/api/licences?asOf=2027-03-01", ""), &licences)
	if len(licences.Rows) != 1 {
		t.Fatalf("licence rows = %d, want 1", len(licences.Rows))
	}
	row := licences.Rows[0]
	if row.SubjectLabel != "Suresh Kumar" || row.SubjectKind != fleet.SubjectDriver {
		t.Errorf("row = %+v, want it attached to the driver by name", row)
	}
	if row.Status.Kind != fleet.StatusDueSoon || row.Status.DaysLeft != 9 {
		t.Errorf("status = %+v, want due soon with 9 days left", row.Status)
	}

	// The licence must not appear on the vehicle question for any lorry.
	if got := do(t, s, "DELETE", "/api/driver?id="+driver.ID, "").Code; got != http.StatusOK {
		t.Errorf("DELETE /api/driver = %d", got)
	}
	decode(t, do(t, s, "GET", "/api/licences", ""), &licences)
	if len(licences.Rows) != 0 {
		t.Errorf("removing the driver left %d licence rows", len(licences.Rows))
	}
}

func TestServerDeleteVehicleAndDocument(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	if got := do(t, s, "DELETE", "/api/document?subject=ka-01-ab-1234&kind=PUC", "").Code; got != http.StatusOK {
		t.Errorf("DELETE /api/document = %d", got)
	}
	if got := do(t, s, "DELETE", "/api/document?subject=KA01AB1234&kind=PUC", "").Code; got != http.StatusNotFound {
		t.Errorf("deleting the same document twice = %d, want 404", got)
	}
	var state stateResponse
	decode(t, do(t, s, "GET", "/api/state", ""), &state)
	if len(state.Documents) != 3 {
		t.Errorf("documents = %d, want 3", len(state.Documents))
	}

	if got := do(t, s, "DELETE", "/api/vehicle?regNo=KA01AB1234", "").Code; got != http.StatusOK {
		t.Errorf("DELETE /api/vehicle = %d", got)
	}
	decode(t, do(t, s, "GET", "/api/state", ""), &state)
	if len(state.Vehicles) != 0 || len(state.Documents) != 0 {
		t.Errorf("removing the vehicle left %d vehicles and %d documents", len(state.Vehicles), len(state.Documents))
	}
	if got := do(t, s, "DELETE", "/api/vehicle?regNo=KA01AB1234", "").Code; got != http.StatusNotFound {
		t.Errorf("deleting the same vehicle twice = %d, want 404", got)
	}
}

func TestServerPolicy(t *testing.T) {
	s := testServer(t)
	seed(t, s)
	// Let the road tax lapse.
	if got := do(t, s, "PUT", "/api/document", `{"vehicleRegNo":"KA01AB1234","driverId":"","kind":"RoadTax","number":"TAX-1","issuedOn":"","expiresOn":"2026-12-31","fileRef":""}`); got.Code != http.StatusOK {
		t.Fatalf("PUT road tax = %d: %s", got.Code, got.Body.String())
	}

	var answer canRunResponse
	decode(t, do(t, s, "GET", "/api/can-run?regNo=KA01AB1234&asOf=2027-03-01", ""), &answer)
	reported := false
	for _, item := range answer.Verdict.Reported {
		if item.Kind == fleet.KindRoadTax {
			reported = true
		}
	}
	if !reported {
		t.Error("an expired road tax must be reported even when it does not block")
	}

	if got := do(t, s, "PUT", "/api/policy", `{"blocking":{"RoadTax":true},"validity":{}}`); got.Code != http.StatusOK {
		t.Fatalf("PUT /api/policy = %d: %s", got.Code, got.Body.String())
	}
	decode(t, do(t, s, "GET", "/api/can-run?regNo=KA01AB1234&asOf=2027-03-01", ""), &answer)
	blocked := false
	for _, item := range answer.Verdict.Blocking {
		if item.Kind == fleet.KindRoadTax {
			blocked = true
		}
	}
	if !blocked {
		t.Errorf("with road tax set to block it must appear in blocking, got %+v", answer.Verdict.Blocking)
	}

	if got := do(t, s, "PUT", "/api/policy", `{"blocking":{"DrivingLicence":true},"validity":{}}`).Code; got != http.StatusBadRequest {
		t.Errorf("a driving licence in the vehicle policy = %d, want 400", got)
	}
	if got := do(t, s, "PUT", "/api/policy", `{"blocking":{},"validity":{"PUC":{"firstIssueMonths":12,"renewalMonths":-3}}}`).Code; got != http.StatusBadRequest {
		t.Errorf("a negative validity period = %d, want 400", got)
	}
}

func TestServerSuggestExpiry(t *testing.T) {
	s := testServer(t)

	var out suggestResponse
	decode(t, do(t, s, "GET", "/api/suggest-expiry?kind=PUC&issuedOn=2026-08-31", ""), &out)
	if !out.Found || out.Suggested != fleet.NewDate(2027, time.February, 28) {
		t.Errorf("suggested %s (found %v), want 2027-02-28", out.Suggested, out.Found)
	}
	if !strings.Contains(out.Note, "Indicative") {
		t.Errorf("note = %q, want the indicative warning", out.Note)
	}

	decode(t, do(t, s, "GET", "/api/suggest-expiry?kind=PUC&issuedOn=2026-08-31&firstIssue=true", ""), &out)
	if !out.Found || out.Suggested != fleet.NewDate(2027, time.August, 31) {
		t.Errorf("suggested %s for a new vehicle, want 2027-08-31", out.Suggested)
	}

	decode(t, do(t, s, "GET", "/api/suggest-expiry?kind=RoadTax&issuedOn=2026-08-31", ""), &out)
	if out.Found {
		t.Error("road tax has no default, so no date should be suggested")
	}

	// An override must flow through to the suggestion.
	if got := do(t, s, "PUT", "/api/policy", `{"blocking":{},"validity":{"PUC":{"firstIssueMonths":12,"renewalMonths":3}}}`).Code; got != http.StatusOK {
		t.Fatalf("PUT /api/policy = %d", got)
	}
	decode(t, do(t, s, "GET", "/api/suggest-expiry?kind=PUC&issuedOn=2026-08-31", ""), &out)
	if out.Suggested != fleet.NewDate(2026, time.November, 30) {
		t.Errorf("suggested %s after the override, want 2026-11-30", out.Suggested)
	}

	if got := do(t, s, "GET", "/api/suggest-expiry?kind=PUC&issuedOn=rubbish", "").Code; got != http.StatusBadRequest {
		t.Errorf("a bad issue date = %d, want 400", got)
	}
	if got := do(t, s, "GET", "/api/suggest-expiry?kind=Pizza&issuedOn=2026-08-31", "").Code; got != http.StatusBadRequest {
		t.Errorf("a bad kind = %d, want 400", got)
	}
}

func TestServerChecklist(t *testing.T) {
	s := testServer(t)

	var lists []fleet.Checklist
	decode(t, do(t, s, "GET", "/api/checklist", ""), &lists)
	if len(lists) != len(fleet.AllKinds()) {
		t.Errorf("got %d checklists, want one per kind", len(lists))
	}

	decode(t, do(t, s, "GET", "/api/checklist?kind=cf", ""), &lists)
	if len(lists) != 1 || lists[0].Kind != fleet.KindFitness {
		t.Fatalf("got %+v, want the fitness checklist", lists)
	}
	if len(lists[0].Items) == 0 || lists[0].Note == "" {
		t.Error("a checklist must carry both items and the honesty note")
	}

	if got := do(t, s, "GET", "/api/checklist?kind=Pizza", "").Code; got != http.StatusBadRequest {
		t.Errorf("an unknown kind = %d, want 400", got)
	}
}

func TestServerCSVExportImportRoundTrip(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	export := do(t, s, "GET", "/api/export.csv", "")
	if export.Code != http.StatusOK {
		t.Fatalf("GET /api/export.csv = %d", export.Code)
	}
	if got := export.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", got)
	}
	if !strings.Contains(export.Header().Get("Content-Disposition"), "fleet-documents.csv") {
		t.Errorf("Content-Disposition = %q", export.Header().Get("Content-Disposition"))
	}
	csvBody := export.Body.String()
	if !strings.Contains(csvBody, "KA01AB1234") || !strings.Contains(csvBody, "2026-03-31") {
		t.Fatalf("the export is missing the records:\n%s", csvBody)
	}

	// Wipe everything, then put it back from the CSV.
	if got := do(t, s, "DELETE", "/api/vehicle?regNo=KA01AB1234", "").Code; got != http.StatusOK {
		t.Fatalf("DELETE /api/vehicle = %d", got)
	}
	recorder := do(t, s, "POST", "/api/import.csv?mode=replace", csvBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/import.csv = %d: %s", recorder.Code, recorder.Body.String())
	}
	var report importResponse
	decode(t, recorder, &report)
	if report.Vehicles != 1 || report.Documents != 4 {
		t.Errorf("report = %+v, want 1 vehicle and 4 documents", report)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v, want none for the vault's own export", report.Warnings)
	}

	var state stateResponse
	decode(t, do(t, s, "GET", "/api/state", ""), &state)
	if len(state.Vehicles) != 1 || state.Vehicles[0].Make != "Tata 407" {
		t.Errorf("vehicles = %+v, want the lorry back with its make", state.Vehicles)
	}
}

func TestServerCSVImportReportsBadRows(t *testing.T) {
	s := testServer(t)
	body := "record_type,reg_no,vehicle_type,doc_kind,expires_on\n" +
		"vehicle,KA01AB1234,goods,,\n" +
		"document,KA01AB1234,,PUC,whenever\n" +
		"document,MH12CD5678,,PUC,2030-01-01\n"

	recorder := do(t, s, "POST", "/api/import.csv?mode=merge", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /api/import.csv = %d: %s", recorder.Code, recorder.Body.String())
	}
	var report importResponse
	decode(t, recorder, &report)
	if report.Vehicles != 1 {
		t.Errorf("vehicles = %d, want the good row imported", report.Vehicles)
	}
	if len(report.Warnings) != 2 {
		t.Errorf("warnings = %v, want one per unusable row", report.Warnings)
	}

	if got := do(t, s, "POST", "/api/import.csv", "no header here\n").Code; got != http.StatusBadRequest {
		t.Errorf("a headerless CSV = %d, want 400", got)
	}
	if got := do(t, s, "POST", "/api/import.csv?mode=sideways", body).Code; got != http.StatusBadRequest {
		t.Errorf("an unknown import mode = %d, want 400", got)
	}
}

func TestServerNextDue(t *testing.T) {
	s := testServer(t)
	seed(t, s)

	var out nextDueResponse
	decode(t, do(t, s, "GET", "/api/next-due?regNo=KA01AB1234&asOf=2027-03-01", ""), &out)
	if !out.Found || out.Document == nil || out.Document.Kind != fleet.KindInsurance {
		t.Fatalf("nextDue = %+v, want the insurance", out)
	}
	if out.Status.Kind != fleet.StatusExpired {
		t.Errorf("status = %+v, want expired", out.Status)
	}
	if got := do(t, s, "GET", "/api/next-due?regNo=KA99ZZ0000", "").Code; got != http.StatusNotFound {
		t.Errorf("an unknown vehicle = %d, want 404", got)
	}
}
