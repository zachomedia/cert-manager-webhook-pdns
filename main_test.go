package main

import (
	"os"
	"testing"

	"github.com/jetstack/cert-manager/test/acme/dns"
)

var (
	zone      = os.Getenv("TEST_ZONE_NAME")
	dnsServer = getEnv("TEST_DNS_SERVER", "8.8.8.8:53")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	fixture := dns.NewFixture(&powerDNSProviderSolver{},
		dns.SetDNSServer(dnsServer),
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/pdns"),
		dns.SetStrict(true),
	)

	fixture.RunConformance(t)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
