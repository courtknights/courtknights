// Command oauthfake is a local fake of the Google and GitHub OAuth2 providers
// used by CourtKnights in development and tests. It implements the redirect
// (authorization code) flow and the Device Authorization Grant (RFC 8628), and
// mirrors each provider's userinfo response shape. See ADR-014.
//
// It accepts any client ID and secret. Never expose it outside a development
// environment.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	baseURL := flag.String("base-url", "http://localhost:8090", "externally reachable URL of the fake, used in device verification URIs")
	codeTTL := flag.Duration("code-ttl", 10*time.Minute, "lifetime of authorization and device codes")
	interval := flag.Duration("device-interval", time.Second, "minimum polling interval announced for the device flow")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newServer(newStore(time.Now, *codeTTL, *interval), *baseURL),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("oauthfake listening on %s (base URL %s)", *addr, *baseURL)
	log.Fatal(srv.ListenAndServe())
}
