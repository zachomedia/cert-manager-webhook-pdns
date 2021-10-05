package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"

	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pdns "github.com/zachomedia/cert-manager-webhook-pdns/provider"
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
	Host            string                    `json:"host"`
	APIKeySecretRef *cmmeta.SecretKeySelector `json:"apiKeySecretRef"`

	// +optional
	TTL int `json:"ttl"`

	// +optional
	Timeout int `json:"timeout"`

	// +optional
	PropagationTimeout int `json:"propagationTimeout"`

	// +optional
	PollingInterval int `json:"pollingInterval"`
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

func (c *powerDNSProviderSolver) validate(cfg *powerDNSProviderConfig) error {
	// Check that the host is defined
	if cfg.Host == "" {
		return errors.New("No PowerDNS host provided")
	}

	// Try to load the API key
	if cfg.APIKeySecretRef.LocalObjectReference.Name == "" {
		return errors.New("No PowerDNS API key provided")
	}

	return nil
}

func (c *powerDNSProviderSolver) provider(cfg *powerDNSProviderConfig, namespace string) (*pdns.DNSProvider, error) {
	if err := c.validate(cfg); err != nil {
		return nil, err
	}

	//c.client.CoreV1().Secrets(namespace).Get("")
	sec, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), cfg.APIKeySecretRef.LocalObjectReference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	secBytes, ok := sec.Data[cfg.APIKeySecretRef.Key]
	if !ok {
		return nil, fmt.Errorf("Key %q not found in secret \"%s/%s\"", cfg.APIKeySecretRef.Key, cfg.APIKeySecretRef.LocalObjectReference.Name, namespace)
	}

	apiKey := string(secBytes)

	// Create provider
	providerConfig := pdns.NewDefaultConfig()

	// Parse host
	host, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, err
	}

	providerConfig.Host = host
	providerConfig.APIKey = apiKey

	if cfg.PropagationTimeout > 0 {
		providerConfig.PropagationTimeout = time.Duration(cfg.PropagationTimeout) * time.Second
	}

	if cfg.PollingInterval > 0 {
		providerConfig.PollingInterval = time.Duration(cfg.PollingInterval) * time.Second
	}

	if cfg.TTL > 0 {
		providerConfig.TTL = cfg.TTL
	}

	if cfg.Timeout > 0 {
		providerConfig.HTTPClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return pdns.NewDNSProviderConfig(providerConfig)
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *powerDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	provider, err := c.provider(&cfg, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	return provider.Present(ch.ResolvedFQDN, ch.Key)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *powerDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	provider, err := c.provider(&cfg, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	return provider.CleanUp(ch.ResolvedFQDN, ch.Key)
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
func loadConfig(cfgJSON *extapi.JSON) (powerDNSProviderConfig, error) {
	cfg := powerDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}
