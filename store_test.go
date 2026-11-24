package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/fleet-doc-expiry-vault/internal/fleet"
)

func TestOpenStoreWithNoFileYet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("opening the store must not create the file until something is saved")
	}
	store.Read(func(f *fleet.Fleet) {
		if len(f.Vehicles) != 0 {
			t.Errorf("a fresh store has %d vehicles, want 0", len(f.Vehicles))
		}
		if !f.Blocking[fleet.KindInsurance] || f.Blocking[fleet.KindRoadTax] {
			t.Errorf("a fresh store must carry the default policy, got %+v", f.Blocking)
		}
	})
}

func TestStoreRoundTripsThroughTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(f *fleet.Fleet) error {
		if _, err := f.PutVehicle(fleet.Vehicle{RegNo: "KA 01 AB 1234", Type: fleet.TypeGoods, Make: "Tata 407"}); err != nil {
			return err
		}
		_, err := f.PutDocument(fleet.Document{
			VehicleRegNo: "KA01AB1234",
			Kind:         fleet.KindInsurance,
			Number:       "POL-9",
			IssuedOn:     fleet.NewDate(2027, time.April, 1),
			ExpiresOn:    fleet.NewDate(2028, time.March, 31),
			FileRef:      "scans/ka01.pdf",
		})
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the data file was not written: %v", err)
	}
	if !strings.Contains(string(raw), `"2028-03-31"`) {
		t.Errorf("dates must be stored readably, got:\n%s", raw)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("the data file should end with a newline so it plays nicely with text tools")
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore (reopen): %v", err)
	}
	reopened.Read(func(f *fleet.Fleet) {
		if len(f.Vehicles) != 1 || f.Vehicles[0].RegNo != "KA01AB1234" {
			t.Fatalf("vehicles = %+v, want the normalised KA01AB1234", f.Vehicles)
		}
		if len(f.Documents) != 1 {
			t.Fatalf("documents = %+v, want 1", f.Documents)
		}
		doc := f.Documents[0]
		if doc.ExpiresOn != fleet.NewDate(2028, time.March, 31) || doc.Number != "POL-9" || doc.FileRef != "scans/ka01.pdf" {
			t.Errorf("document = %+v, want the values that went in", doc)
		}
	})
}

func TestStoreSavesAtomicallyAndLeavesNoLitter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := store.Write(func(f *fleet.Fleet) error {
			_, err := f.PutVehicle(fleet.Vehicle{RegNo: "KA01AB1234", Type: fleet.TypeGoods})
			return err
		}); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "vault.json" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("directory holds %v, want only vault.json — no temporary files left behind", names)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("data file mode = %o, want 600 — these are the owner's business records", mode)
	}
}

func TestStoreWriteKeepsTheFileWhenTheEditIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(f *fleet.Fleet) error {
		_, err := f.PutVehicle(fleet.Vehicle{RegNo: "KA01AB1234", Type: fleet.TypeGoods})
		return err
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// A document with no expiry date is refused by the domain.
	err = store.Write(func(f *fleet.Fleet) error {
		_, err := f.PutDocument(fleet.Document{VehicleRegNo: "KA01AB1234", Kind: fleet.KindPUC})
		return err
	})
	if err == nil {
		t.Fatal("a document with no expiry date must be refused")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(before) != string(after) {
		t.Error("a rejected edit must not touch the data file")
	}
	store.Read(func(f *fleet.Fleet) {
		if len(f.Documents) != 0 {
			t.Errorf("a rejected edit left %d documents in memory", len(f.Documents))
		}
	})
}

func TestOpenStoreRejectsRubbish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	if err := os.WriteFile(path, []byte("this is not JSON"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("OpenStore must refuse a file it cannot parse rather than silently starting empty")
	}
}

func TestOpenStoreTreatsAnEmptyFileAsAnEmptyFleet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(f *fleet.Fleet) {
		if f.Blocking == nil || f.Validity == nil {
			t.Error("an empty file must still give a fleet with the default policy")
		}
	})
}

func TestOpenStoreNormalisesAHandEditedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	hand := `{
	  "vehicles": [{"regNo": "ka 01 ab 1234", "type": "goods"}],
	  "documents": [{"vehicleRegNo": "ka-01-ab-1234", "kind": "Insurance", "expiresOn": "31-03-2028"}]
	}`
	if err := os.WriteFile(path, []byte(hand), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	store.Read(func(f *fleet.Fleet) {
		if f.Vehicles[0].RegNo != "KA01AB1234" || f.Documents[0].VehicleRegNo != "KA01AB1234" {
			t.Errorf("registration numbers not normalised: %q / %q", f.Vehicles[0].RegNo, f.Documents[0].VehicleRegNo)
		}
		if f.Documents[0].ExpiresOn != fleet.NewDate(2028, time.March, 31) {
			t.Errorf("ExpiresOn = %s, want 2028-03-31 read from the day-first form", f.Documents[0].ExpiresOn)
		}
		if !f.Blocking[fleet.KindPUC] {
			t.Error("a file with no policy section must pick up the defaults")
		}
	})
}

func TestReplaceRecordsKeepsThePolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.Write(func(f *fleet.Fleet) error {
		if _, err := f.PutVehicle(fleet.Vehicle{RegNo: "KA01AB1234", Type: fleet.TypeGoods}); err != nil {
			return err
		}
		return f.SetBlocking(fleet.KindRoadTax, true)
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	replacement := fleet.New()
	if _, err := replacement.PutVehicle(fleet.Vehicle{RegNo: "MH12CD5678", Type: fleet.TypePassenger}); err != nil {
		t.Fatalf("PutVehicle: %v", err)
	}
	if err := store.ReplaceRecords(replacement); err != nil {
		t.Fatalf("ReplaceRecords: %v", err)
	}

	store.Read(func(f *fleet.Fleet) {
		if len(f.Vehicles) != 1 || f.Vehicles[0].RegNo != "MH12CD5678" {
			t.Errorf("vehicles = %+v, want only MH12CD5678", f.Vehicles)
		}
		if !f.Blocking[fleet.KindRoadTax] {
			t.Error("replacing the records must keep the owner's own blocking policy")
		}
	})
}
