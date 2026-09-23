// Command engine runs the FlowForge durable workflow engine over HTTP. This entry
// point uses the in-memory store; the Postgres-backed store (same interface) is wired
// in via FLOWFORGE_DB when running against the docker-compose stack.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/parag-labs/flowforge/services/engine-go/engine"
)

func main() {
	addr := os.Getenv("FLOWFORGE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	leaseSeconds := 30
	if v := os.Getenv("FLOWFORGE_LEASE_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			leaseSeconds = n
		}
	}

	store := engine.NewStore(leaseSeconds)
	api := engine.NewAPI(store)

	log.Printf("flowforge engine listening on %s (lease %ds)", addr, leaseSeconds)
	if err := http.ListenAndServe(addr, api.Routes()); err != nil {
		log.Fatal(err)
	}
}
