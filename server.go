package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/sreenivas-sadhu-prabhakara/fleet-doc-expiry-vault/internal/fleet"
)

// maxBodyBytes caps every request body. The UI never sends anything large, and
// a CSV of a few hundred vehicles is well inside this.
const maxBodyBytes = 4 << 20 // 4 MiB

// Server wires the local web UI and the JSON API over one Store. Everything is
// same-origin: the UI is served from the binary and talks to /api on the same
// loopback address.
type Server struct {
	store *Store
	ui    fs.FS
	mux   *http.ServeMux
}

// NewServer builds the router.
func NewServer(store *Store, ui fs.FS) *Server {
	s := &Server{store: store, ui: ui, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A local tool still sets the headers that stop a stray page in another
	// tab from reading the fleet's data.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.Handle("GET /", http.FileServerFS(s.ui))

	s.mux.HandleFunc("GET /api/state", s.handleState)
	s.mux.HandleFunc("GET /api/board", s.handleBoard)
	s.mux.HandleFunc("GET /api/licences", s.handleLicences)
	s.mux.HandleFunc("GET /api/can-run", s.handleCanRun)
	s.mux.HandleFunc("GET /api/next-due", s.handleNextDue)
	s.mux.HandleFunc("GET /api/checklist", s.handleChecklist)
	s.mux.HandleFunc("GET /api/suggest-expiry", s.handleSuggestExpiry)
	s.mux.HandleFunc("GET /api/export.csv", s.handleExportCSV)

	s.mux.HandleFunc("PUT /api/vehicle", s.handlePutVehicle)
	s.mux.HandleFunc("DELETE /api/vehicle", s.handleDeleteVehicle)
	s.mux.HandleFunc("PUT /api/driver", s.handlePutDriver)
	s.mux.HandleFunc("DELETE /api/driver", s.handleDeleteDriver)
	s.mux.HandleFunc("PUT /api/document", s.handlePutDocument)
	s.mux.HandleFunc("DELETE /api/document", s.handleDeleteDocument)
	s.mux.HandleFunc("PUT /api/policy", s.handlePutPolicy)
	s.mux.HandleFunc("POST /api/import.csv", s.handleImportCSV)
}

// ---------- reads ----------

// kindInfo describes one document kind to the UI, so the labels, the blocking
// policy and the indicative validity periods are stated in exactly one place.
type kindInfo struct {
	Kind            fleet.DocKind        `json:"kind"`
	Label           string               `json:"label"`
	BelongsToDriver bool                 `json:"belongsToDriver"`
	Blocks          bool                 `json:"blocks"`
	Validity        fleet.ValidityPeriod `json:"validity"`
	ValidityNote    string               `json:"validityNote"`
}

type stateResponse struct {
	Today     fleet.Date       `json:"today"`
	DataFile  string           `json:"dataFile"`
	Vehicles  []fleet.Vehicle  `json:"vehicles"`
	Drivers   []fleet.Driver   `json:"drivers"`
	Documents []fleet.Document `json:"documents"`
	Kinds     []kindInfo       `json:"kinds"`
	Tiers     []int            `json:"tiers"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	out := stateResponse{
		Today:    fleet.Today(),
		DataFile: s.store.Path(),
		Tiers:    []int{fleet.TierNoticeDays, fleet.TierWarnDays, fleet.TierUrgentDays},
	}
	s.store.Read(func(f *fleet.Fleet) {
		out.Vehicles = append(out.Vehicles, f.Vehicles...)
		out.Drivers = append(out.Drivers, f.Drivers...)
		out.Documents = append(out.Documents, f.Documents...)
		for _, kind := range fleet.AllKinds() {
			out.Kinds = append(out.Kinds, kindInfo{
				Kind:            kind,
				Label:           kind.Label(),
				BelongsToDriver: kind.BelongsToDriver(),
				Blocks:          f.Blocking[kind],
				Validity:        f.Validity[kind],
				ValidityNote:    fleet.ValidityNote(kind),
			})
		}
	})
	if out.Vehicles == nil {
		out.Vehicles = []fleet.Vehicle{}
	}
	if out.Drivers == nil {
		out.Drivers = []fleet.Driver{}
	}
	if out.Documents == nil {
		out.Documents = []fleet.Document{}
	}
	writeJSON(w, http.StatusOK, out)
}

type boardResponse struct {
	AsOf   fleet.Date       `json:"asOf"`
	Rows   []fleet.BoardRow `json:"rows"`
	Counts fleet.Counts     `json:"counts"`
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out := boardResponse{AsOf: asOf, Rows: []fleet.BoardRow{}}
	s.store.Read(func(f *fleet.Fleet) {
		out.Rows = append(out.Rows, f.FleetBoard(asOf)...)
	})
	out.Counts = fleet.Summarise(out.Rows)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleLicences(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out := boardResponse{AsOf: asOf, Rows: []fleet.BoardRow{}}
	s.store.Read(func(f *fleet.Fleet) {
		out.Rows = append(out.Rows, f.LicenceBoard(asOf)...)
	})
	out.Counts = fleet.Summarise(out.Rows)
	writeJSON(w, http.StatusOK, out)
}

type canRunResponse struct {
	Vehicle fleet.Vehicle    `json:"vehicle"`
	Verdict fleet.RunVerdict `json:"verdict"`
	Rows    []fleet.BoardRow `json:"rows"`
	NextDue *fleet.Document  `json:"nextDue"`
}

func (s *Server) handleCanRun(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	regNo := r.URL.Query().Get("regNo")
	if strings.TrimSpace(regNo) == "" {
		writeError(w, http.StatusBadRequest, errors.New("which vehicle? pass regNo"))
		return
	}

	var out canRunResponse
	found := false
	s.store.Read(func(f *fleet.Fleet) {
		vehicle, ok := f.Vehicle(regNo)
		if !ok {
			return
		}
		found = true
		verdict, _ := f.CanRunToday(regNo, asOf)
		out.Vehicle = vehicle
		out.Verdict = verdict
		out.Rows = f.VehicleBoard(regNo, asOf)
		if next, ok := f.NextDue(regNo); ok {
			out.NextDue = &next
		}
	})
	if !found {
		writeError(w, http.StatusNotFound, fmt.Errorf("no vehicle registered as %q is on file", regNo))
		return
	}
	if out.Rows == nil {
		out.Rows = []fleet.BoardRow{}
	}
	writeJSON(w, http.StatusOK, out)
}

type nextDueResponse struct {
	RegNo    string          `json:"regNo"`
	Found    bool            `json:"found"`
	Document *fleet.Document `json:"document"`
	Status   fleet.DocStatus `json:"status"`
}

func (s *Server) handleNextDue(w http.ResponseWriter, r *http.Request) {
	asOf, err := asOfParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	regNo := r.URL.Query().Get("regNo")
	out := nextDueResponse{RegNo: fleet.NormaliseRegNo(regNo)}
	known := false
	s.store.Read(func(f *fleet.Fleet) {
		if _, ok := f.Vehicle(regNo); !ok {
			return
		}
		known = true
		if doc, ok := f.NextDue(regNo); ok {
			out.Found = true
			out.Document = &doc
			out.Status = fleet.Status(doc.ExpiresOn, asOf)
		}
	})
	if !known {
		writeError(w, http.StatusNotFound, fmt.Errorf("no vehicle registered as %q is on file", regNo))
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleChecklist(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("kind")
	if strings.TrimSpace(raw) == "" {
		writeJSON(w, http.StatusOK, fleet.AllChecklists())
		return
	}
	kind, err := fleet.ParseDocKind(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	list, err := fleet.RenewalChecklist(kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, []fleet.Checklist{list})
}

type suggestResponse struct {
	Suggested fleet.Date `json:"suggested"`
	Found     bool       `json:"found"`
	Note      string     `json:"note"`
}

func (s *Server) handleSuggestExpiry(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind, err := fleet.ParseDocKind(query.Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	issuedOn, err := fleet.ParseDate(query.Get("issuedOn"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	firstIssue := query.Get("firstIssue") == "true"

	out := suggestResponse{Note: fleet.ValidityNote(kind)}
	s.store.Read(func(f *fleet.Fleet) {
		out.Suggested, out.Found = f.SuggestExpiry(kind, issuedOn, firstIssue)
	})
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fleet-documents.csv"`)
	// The status line goes out with the first byte, so a write that fails
	// part-way can only stop. Nothing is lost: the data file is untouched by an
	// export, and the owner simply downloads it again.
	s.store.Read(func(f *fleet.Fleet) {
		_ = fleet.WriteCSV(w, f)
	})
}

// ---------- writes ----------

func (s *Server) handlePutVehicle(w http.ResponseWriter, r *http.Request) {
	var in fleet.Vehicle
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var saved fleet.Vehicle
	if err := s.store.Write(func(f *fleet.Fleet) error {
		var err error
		saved, err = f.PutVehicle(in)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteVehicle(w http.ResponseWriter, r *http.Request) {
	regNo := r.URL.Query().Get("regNo")
	removed := false
	if err := s.store.Write(func(f *fleet.Fleet) error {
		removed = f.RemoveVehicle(regNo)
		if !removed {
			return fmt.Errorf("no vehicle registered as %q is on file", regNo)
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": regNo})
}

func (s *Server) handlePutDriver(w http.ResponseWriter, r *http.Request) {
	var in fleet.Driver
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var saved fleet.Driver
	if err := s.store.Write(func(f *fleet.Fleet) error {
		var err error
		saved, err = f.PutDriver(in)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteDriver(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if err := s.store.Write(func(f *fleet.Fleet) error {
		if !f.RemoveDriver(id) {
			return fmt.Errorf("no driver with ID %q is on file", id)
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": id})
}

func (s *Server) handlePutDocument(w http.ResponseWriter, r *http.Request) {
	var in fleet.Document
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var saved fleet.Document
	if err := s.store.Write(func(f *fleet.Fleet) error {
		var err error
		saved, err = f.PutDocument(in)
		return err
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind, err := fleet.ParseDocKind(query.Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	subject := query.Get("subject")
	if err := s.store.Write(func(f *fleet.Fleet) error {
		if !f.RemoveDocument(subject, kind) {
			return fmt.Errorf("no %s is on file for %q", kind.Label(), subject)
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": string(kind), "subject": subject})
}

type policyRequest struct {
	Blocking map[fleet.DocKind]bool                 `json:"blocking"`
	Validity map[fleet.DocKind]fleet.ValidityPeriod `json:"validity"`
}

func (s *Server) handlePutPolicy(w http.ResponseWriter, r *http.Request) {
	var in policyRequest
	if err := decodeBody(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.Write(func(f *fleet.Fleet) error {
		for kind, blocks := range in.Blocking {
			if err := f.SetBlocking(kind, blocks); err != nil {
				return err
			}
		}
		for kind, period := range in.Validity {
			if err := f.SetValidity(kind, period); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.handleState(w, r)
}

type importResponse struct {
	Mode      string       `json:"mode"`
	Vehicles  int          `json:"vehicles"`
	Drivers   int          `json:"drivers"`
	Documents int          `json:"documents"`
	Warnings  []string     `json:"warnings"`
	Counts    fleet.Counts `json:"counts"`
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("that file is too large to import: %w", err))
		return
	}
	imported, warnings, err := fleet.ReadCSV(strings.NewReader(string(body)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "merge"
	}

	out := importResponse{Mode: mode, Warnings: warnings}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}

	switch mode {
	case "replace":
		if err := s.store.ReplaceRecords(imported); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	case "merge":
		if err := s.store.Write(func(f *fleet.Fleet) error {
			for _, v := range imported.Vehicles {
				if _, err := f.PutVehicle(v); err != nil {
					out.Warnings = append(out.Warnings, err.Error())
				}
			}
			for _, d := range imported.Drivers {
				if _, err := f.PutDriver(d); err != nil {
					out.Warnings = append(out.Warnings, err.Error())
				}
			}
			for _, doc := range imported.Documents {
				if _, err := f.PutDocument(doc); err != nil {
					out.Warnings = append(out.Warnings, err.Error())
				}
			}
			return nil
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("mode %q is not \"merge\" or \"replace\"", mode))
		return
	}

	asOf := fleet.Today()
	s.store.Read(func(f *fleet.Fleet) {
		out.Vehicles = len(f.Vehicles)
		out.Drivers = len(f.Drivers)
		out.Documents = len(f.Documents)
		out.Counts = fleet.Summarise(f.FleetBoard(asOf))
	})
	writeJSON(w, http.StatusOK, out)
}

// ---------- plumbing ----------

// asOfParam reads the day being asked about. The UI normally leaves it off and
// gets today, but it can be set to look ahead ("will this lorry still be legal
// on the 15th?") or back.
func asOfParam(r *http.Request) (fleet.Date, error) {
	raw := r.URL.Query().Get("asOf")
	if strings.TrimSpace(raw) == "" {
		return fleet.Today(), nil
	}
	return fleet.ParseDate(raw)
}

func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("could not read the request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
