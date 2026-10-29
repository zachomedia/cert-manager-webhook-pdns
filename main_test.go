package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	dns "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone      = os.Getenv("TEST_ZONE_NAME")
	dnsServer = getEnv("TEST_DNS_SERVER", "8.8.8.8:53")
)

func test(t *testing.T, manifestPath string) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	fixture := dns.NewFixture(&powerDNSProviderSolver{},
		dns.SetDNSServer(dnsServer),
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath(manifestPath),
		dns.SetStrict(true),
	)

	fixture.RunConformance(t)
}

func TestNoProxyNoTLS(t *testing.T) {
	test(t, "_out/testdata/no-tls")
}

func TestNoProxyNoTLSAuthHdr(t *testing.T) {
	test(t, "_out/testdata/no-tls-auth-hdr")
}

func TestNoProxyTLS(t *testing.T) {
	test(t, "_out/testdata/tls")
}

func TestNoProxyTLSAuthHdr(t *testing.T) {
	test(t, "_out/testdata/tls-auth-hdr")
}

func TestProxyNoTLS(t *testing.T) {
	test(t, "_out/testdata/no-tls-with-proxy")
}

func TestProxyTLS(t *testing.T) {
	test(t, "_out/testdata/tls-with-proxy")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func TestLoadClientTLSConfigValid(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	cert := server.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certFile, keyFile := filepath.Join(t.TempDir(), "cert.pem"), filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadClientTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatal("expected a TLS config with one client certificate")
	}
}

func TestLoadClientTLSConfig(t *testing.T) {
	for _, tt := range []struct {
		name, cert, key string
		wantErr         bool
	}{
		{name: "disabled"},
		{name: "missing key", cert: "cert.pem", wantErr: true},
		{name: "missing cert", key: "key.pem", wantErr: true},
		{name: "missing files", cert: "missing-cert.pem", key: "missing-key.pem", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadClientTLSConfig(tt.cert, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg != nil {
				t.Fatal("expected no TLS config")
			}
		})
	}
}

func TestIsAllowedZones(t *testing.T) {
	cfg := powerDNSProviderConfig{
		AllowedZones: []string{"example.com.", "example.org."},
	}

	tests := []struct {
		zone    string
		matched bool
	}{
		{"foo.example.com.", true},
		{"foo.example.net.", false},
		{"example.com.", true},
		{"notexample.com.", false},
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			match := cfg.IsAllowedZone(tt.zone)
			if match != tt.matched {
				t.Errorf("Unexpected IsAllowedZone(%s) = %t, expected %t", tt.zone, match, tt.matched)
			}
		})
	}
}
