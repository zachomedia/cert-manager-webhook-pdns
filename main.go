package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/joeig/go-powerdns/v3"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&powerDNSProviderSolver{},
	)
}

// powerDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type powerDNSProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client *kubernetes.Clientset
}

// customDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type powerDNSProviderConfig struct {
	// Host is the Base URL (e.g. https://dns.example.ca) of the PowerDNS API.
	Host string `json:"host"`

	// APIKeySecretRef contains the reference information for the Kubernetes
	// secret which contains the PowerDNS API Key.
	APIKeySecretRef *cmmeta.SecretKeySelector `json:"apiKeySecretRef"`

	// ServerID is the server ID in the PowerDNS API.
	// When unset, defaults to "localhost".
	ServerID string `json:"serverID"`

	// Headers are additional headers added to requests to the
	// PowerDNS API server.
	Headers map[string]string `json:"headers"`

	// CABundle is a PEM encoded CA bundle which will be used in
	// certificate validation when connecting to the PowerDNS server.
	//
	// When left blank, the default system store will be used.
	//
	// +optional
	CABundle []byte `json:"caBundle"`

	// TTL is the time-to-live value of the inserted DNS records.
	//
	// +optional
	TTL int `json:"ttl"`

	// Timeout is the timeout value for requests to the PowerDNS API.
	// The value is specified in seconds.
	//
	// +optional
	Timeout int `json:"timeout"`

	// AllowedZones is the list of zones that may be edited. If the list is
	// empty, all zones are permitted.
	AllowedZones []string `json:"allowed-zones"`
}

// IsAllowedZone checks if the webhook is allowed to edit the given zone, per
// AllowedZones setting. All zones allowed if AllowedZones is empty (the default setting)
func (cfg powerDNSProviderConfig) IsAllowedZone(zone string) bool {
	if len(cfg.AllowedZones) == 0 {
		return true
	}

	for _, allowed := range cfg.AllowedZones {
		if zone == allowed || strings.HasSuffix(zone, "."+allowed) {
			return true
		}
	}
	return false
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *powerDNSProviderSolver) Name() string {
	return "pdns"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *powerDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	ctx := context.Background()

	klog.InfoS("Presenting challenge", "dnsName", ch.DNSName, "resolvedZone", ch.ResolvedZone, "resolvedFQDN", ch.ResolvedFQDN)

	provider, cfg, err := c.init(ch.Config, ch.ResourceNamespace)
	if err != nil {
		return fmt.Errorf("failed initializing powerdns provider: %v", err)
	}

	if !cfg.IsAllowedZone(ch.ResolvedZone) {
		return fmt.Errorf("zone %s may not be edited per config (allowed zones are %v)", ch.ResolvedZone, cfg.AllowedZones)
	}

	records, err := c.getExistingRecords(ctx, provider, ch.ResolvedZone, ch.ResolvedFQDN)
	if err != nil {
		return fmt.Errorf("failed loading existing records for %s in domain %s: %v", ch.ResolvedFQDN, ch.ResolvedZone, err)
	}

	// Add the record, only if it doesn't exist already
	content := quote(ch.Key)
	if _, ok := findRecord(records, content); !ok {
		disabled := false
		records = append(records, powerdns.Record{Disabled: &disabled, Content: &content})
	}

	txtType := powerdns.RRTypeTXT
	ttl := uint32(cfg.TTL)
	changeType := powerdns.ChangeTypeReplace
	rrset := powerdns.RRset{
		Name:       &ch.ResolvedFQDN,
		Type:       &txtType,
		TTL:        &ttl,
		ChangeType: &changeType,
		Records:    records,
	}

	return provider.Records.Patch(ctx, ch.ResolvedZone, &powerdns.RRsets{Sets: []powerdns.RRset{rrset}})
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *powerDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	ctx := context.Background()

	klog.InfoS("Cleaning challenge", "dnsName", ch.DNSName, "resolvedZone", ch.ResolvedZone, "resolvedFQDN", ch.ResolvedFQDN)

	provider, cfg, err := c.init(ch.Config, ch.ResourceNamespace)
	if err != nil {
		return fmt.Errorf("failed initializing powerdns provider: %v", err)
	}

	records, err := c.getExistingRecords(ctx, provider, ch.ResolvedZone, ch.ResolvedFQDN)
	if err != nil {
		return fmt.Errorf("failed loading existing records for %s in domain %s: %v", ch.ResolvedFQDN, ch.ResolvedZone, err)
	}

	content := quote(ch.Key)
	if indx, ok := findRecord(records, content); ok {
		records = append(records[:indx], records[indx+1:]...)
	}

	txtType := powerdns.RRTypeTXT
	ttl := uint32(cfg.TTL)
	changeType := powerdns.ChangeTypeReplace
	rrset := powerdns.RRset{
		Name:       &ch.ResolvedFQDN,
		Type:       &txtType,
		TTL:        &ttl,
		ChangeType: &changeType,
		Records:    records,
	}

	return provider.Records.Patch(ctx, ch.ResolvedZone, &powerdns.RRsets{Sets: []powerdns.RRset{rrset}})
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *powerDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl
	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *apiextensionsv1.JSON) (*powerDNSProviderConfig, error) {
	cfg := &powerDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *powerDNSProviderSolver) validate(cfg *powerDNSProviderConfig) error {
	// Check that the host is defined
	if cfg.Host == "" {
		return errors.New("no PowerDNS host provided")
	}

	// Try to load the API key
	if cfg.APIKeySecretRef.LocalObjectReference.Name == "" {
		return errors.New("no PowerDNS API key provided")
	}

	return nil
}

func (c *powerDNSProviderSolver) init(config *apiextensionsv1.JSON, namespace string) (*powerdns.Client, *powerDNSProviderConfig, error) {
	// Load and validate the configuration
	cfg, err := loadConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed parsing provider config: %v", err)
	}

	if err := c.validate(cfg); err != nil {
		return nil, nil, fmt.Errorf("failed validating config: %v", err)
	}

	// Load the API key secret
	sec, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), cfg.APIKeySecretRef.LocalObjectReference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, cfg, fmt.Errorf("failed loading api key secret %s/%s: %v", namespace, cfg.APIKeySecretRef.LocalObjectReference.Name, err)
	}

	secBytes, ok := sec.Data[cfg.APIKeySecretRef.Key]
	if !ok {
		return nil, cfg, fmt.Errorf("key %q not found in secret \"%s/%s\"", cfg.APIKeySecretRef.Key, cfg.APIKeySecretRef.LocalObjectReference.Name, namespace)
	}

	apiKey := string(secBytes)

	// Set the server ID if it's unset
	if cfg.ServerID == "" {
		cfg.ServerID = "localhost"
	}

	// Create the client
	httpClient := &http.Client{}

	// If the timeout is configured, then set the timeout
	if cfg.Timeout > 0 {
		httpClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	// If a caBundle is provided, then use it
	if len(cfg.CABundle) > 0 {
		caBundle := x509.NewCertPool()
		if ok := caBundle.AppendCertsFromPEM(cfg.CABundle); !ok {
			return nil, cfg, fmt.Errorf("failed to load certificate(s) from CA bundle")
		}

		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			RootCAs: caBundle,
		}

		httpClient.Transport = transport
	}

	// Add request headers
	headers := map[string]string{
		"X-API-Key":    apiKey,
		"Content-Type": "application/json",
	}
	maps.Copy(headers, cfg.Headers)

	return powerdns.NewClient(cfg.Host, cfg.ServerID, headers, httpClient), cfg, nil
}

func (c *powerDNSProviderSolver) getExistingRecords(ctx context.Context, provider *powerdns.Client, domain, name string) ([]powerdns.Record, error) {
	// Find existing records
	zone, err := provider.Zones.Get(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed loading zone %s: %v", domain, err)
	}

	if rrset := findRRSet(zone.RRsets, powerdns.RRTypeTXT, name); rrset != nil {
		return rrset.Records, nil
	}

	return []powerdns.Record{}, nil
}
