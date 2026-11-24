// Command fleet-doc-expiry-vault keeps a small transport fleet's statutory
// documents in one place and answers the question that matters at the gate:
// can this vehicle legally run today?
//
// It is one binary with no dependencies. It serves a small web interface on
// the loopback address and stores everything in a plain JSON file on the same
// machine. It makes no outbound network connections of any kind: there is no
// telemetry, no update check, and no link to any government portal.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

//go:embed web
var webFiles embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on; loopback only unless you deliberately change it")
	data := flag.String("data", "./fleet-doc-expiry-vault-data.json", "path to the local data file")
	flag.Parse()

	if err := run(*addr, *data); err != nil {
		log.Fatalf("fleet-doc-expiry-vault: %v", err)
	}
}

func run(addr, data string) error {
	store, err := OpenStore(data)
	if err != nil {
		return err
	}

	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		return fmt.Errorf("could not open the embedded interface: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           NewServer(store, ui),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	out := log.New(os.Stdout, "", 0)
	out.Printf("Fleet document expiry vault")
	out.Printf("  open      http://%s", listener.Addr().String())
	out.Printf("  data file %s", store.Path())
	out.Printf("  offline   nothing is sent anywhere; stop with Ctrl-C")

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
