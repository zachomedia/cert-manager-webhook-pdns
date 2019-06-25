// Package pdns implements a DNS provider for solving the DNS-01 challenge using PowerDNS nameserver.
package pdns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-acme/lego/challenge/dns01"
	"github.com/go-acme/lego/log"
	"github.com/go-acme/lego/platform/config/env"
)

// Config is used to configure the creation of the DNSProvider
type Config struct {
	APIKey             string
	Host               *url.URL
	PropagationTimeout time.Duration
	PollingInterval    time.Duration
	TTL                int
	HTTPClient         *http.Client
}

// NewDefaultConfig returns a default configuration for the DNSProvider
func NewDefaultConfig() *Config {
	return &Config{
		TTL:                120,
		PropagationTimeout: 120*time.Second,
		PollingInterval:    2*time.Second,
		HTTPClient: &http.Client{
			Timeout: 30*time.Second,
		},
	}
}

// DNSProvider is an implementation of the acme.ChallengeProvider interface
type DNSProvider struct {
	apiVersion int
	config     *Config
}

// NewDNSProvider returns a DNSProvider instance configured for pdns.
// Credentials must be passed in the environment variable:
// PDNS_API_URL and PDNS_API_KEY.
func NewDNSProvider() (*DNSProvider, error) {
	values, err := env.Get("PDNS_API_KEY", "PDNS_API_URL")
	if err != nil {
		return nil, fmt.Errorf("pdns: %v", err)
	}

	hostURL, err := url.Parse(values["PDNS_API_URL"])
	if err != nil {
		return nil, fmt.Errorf("pdns: %v", err)
	}

	config := NewDefaultConfig()
	config.Host = hostURL
	config.APIKey = values["PDNS_API_KEY"]

	return NewDNSProviderConfig(config)
}

// NewDNSProviderConfig return a DNSProvider instance configured for pdns.
func NewDNSProviderConfig(config *Config) (*DNSProvider, error) {
	if config == nil {
		return nil, errors.New("pdns: the configuration of the DNS provider is nil")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("pdns: API key missing")
	}

	if config.Host == nil || config.Host.Host == "" {
		return nil, fmt.Errorf("pdns: API URL missing")
	}

	d := &DNSProvider{config: config}

	apiVersion, err := d.getAPIVersion()
	if err != nil {
		log.Warnf("pdns: failed to get API version %v", err)
	}
	d.apiVersion = apiVersion

	return d, nil
}

// Timeout returns the timeout and interval to use when checking for DNS
// propagation. Adjusting here to cope with spikes in propagation times.
func (d *DNSProvider) Timeout() (timeout, interval time.Duration) {
	return d.config.PropagationTimeout, d.config.PollingInterval
}

// Present creates a TXT record to fulfill the dns-01 challenge
func (d *DNSProvider) Present(fqdn, value string) error {
	zone, err := d.getHostedZone(fqdn)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}

	name := fqdn

	// pre-v1 API wants non-fqdn
	if d.apiVersion == 0 {
		name = dns01.UnFqdn(fqdn)
	}

	rec := Record{
		Content:  "\"" + value + "\"",
		Disabled: false,

		// pre-v1 API
		Type: "TXT",
		Name: name,
		TTL:  d.config.TTL,
	}

	// Look for existing records.
	existingRrSet, err := d.findTxtRecord(fqdn)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}

	// merge the existing and new records
	var records []Record
	if existingRrSet != nil {
		records = existingRrSet.Records
	}
	records = append(records, rec)

	rrsets := rrSets{
		RRSets: []rrSet{
			{
				Name:       name,
				ChangeType: "REPLACE",
				Type:       "TXT",
				Kind:       "Master",
				TTL:        d.config.TTL,
				Records:    records,
			},
		},
	}

	body, err := json.Marshal(rrsets)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}

	_, err = d.sendRequest(http.MethodPatch, zone.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}
	return nil
}

// CleanUp removes the TXT record matching the specified parameters
func (d *DNSProvider) CleanUp(fqdn, value string) error {
	zone, err := d.getHostedZone(fqdn)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}

	set, err := d.findTxtRecord(fqdn)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}
	if set == nil {
		return fmt.Errorf("pdns: no existing record found for %s", fqdn)
	}

	for indx, item := range set.Records {
		// Remove this item
		if item.Content == "\""+value+"\"" {
			set.Records[indx] = set.Records[len(set.Records)-1]
			set.Records = set.Records[:len(set.Records)-1]
			break
		}
	}

	rrsets := rrSets{
		RRSets: []rrSet{
			{
				Name:       set.Name,
				ChangeType: "REPLACE",
				Type:       set.Type,
				Kind:       set.Kind,
				TTL:        set.TTL,
				Records:    set.Records,
			},
		},
	}

	body, err := json.Marshal(rrsets)
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}

	_, err = d.sendRequest(http.MethodPatch, zone.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pdns: %v", err)
	}
	return nil
}
