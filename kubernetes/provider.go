package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/go-homedir"
	"k8s.io/apimachinery/pkg/api/meta"
	k8sresource "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Provider() *schema.Provider {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"apply_retry_count": {
				Type:        schema.TypeInt,
				Optional:    true,
				DefaultFunc: func() (interface{}, error) { return 1, nil },
				Description: "Defines the number of attempts any create/update action will take",
			},
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_HOST", ""),
				Description: "The hostname (in form of URI) of Kubernetes master.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_USER", ""),
				Description: "The username to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_PASSWORD", ""),
				Description: "The password to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
			},
			"insecure": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_INSECURE", false),
				Description: "Whether server should be accessed without verifying the TLS certificate.",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLIENT_CERT_DATA", ""),
				Description: "PEM-encoded client certificate for TLS authentication.",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLIENT_KEY_DATA", ""),
				Description: "PEM-encoded client certificate key for TLS authentication.",
			},
			"cluster_ca_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLUSTER_CA_CERT_DATA", ""),
				Description: "PEM-encoded root certificates bundle for TLS authentication.",
			},
			"config_path": {
				Type:     schema.TypeString,
				Optional: true,
				DefaultFunc: schema.MultiEnvDefaultFunc(
					[]string{
						"KUBE_CONFIG",
						"KUBECONFIG",
					},
					"~/.kube/config"),
				Description: "Path to the kube config file, defaults to ~/.kube/config",
			},
			"config_context": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX", ""),
			},
			"config_context_auth_info": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX_AUTH_INFO", ""),
				Description: "",
			},
			"config_context_cluster": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX_CLUSTER", ""),
				Description: "",
			},
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_TOKEN", ""),
				Description: "Token to authentifcate an service account",
			},
			"load_config_file": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_LOAD_CONFIG_FILE", true),
				Description: "Load local kubeconfig.",
			},
			"exec": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_version": {
							Type:     schema.TypeString,
							Required: true,
						},
						"command": {
							Type:     schema.TypeString,
							Required: true,
						},
						"env": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"args": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
				Description: "",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"kubectl-query_server_version": dataSourceKubectlServerVersion(),
			"kubectl-query_services":       dataSourceKubectlServices(),
			"kubectl-query_pods":       	dataSourceKubectlPods(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"kubectl-query_server_version": resourceKubectlServerVersion(),
			"kubectl-query_services":       resourceKubectlServices(),
			"kubectl-query_pods":       	resourceKubectlPods(),
		},
	}

	p.ConfigureContextFunc = func(context context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		terraformVersion := p.TerraformVersion
		if terraformVersion == "" {
			// Terraform 0.12 introduced this field to the protocol
			// We can therefore assume that if it's missing it's 0.10 or 0.11
			terraformVersion = "0.11+compatible"
		}
		return providerConfigure(d, terraformVersion)
	}

	return p
}

type KubeProvider struct {
	MainClientset       *kubernetes.Clientset
	RestConfig          restclient.Config
	AggregatorClientset *aggregator.Clientset
}

var _ k8sresource.RESTClientGetter = &KubeProvider{}

func (p *KubeProvider) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return nil
}

func (p *KubeProvider) ToRESTConfig() (*restclient.Config, error) {
	return &p.RestConfig, nil
}

func (p *KubeProvider) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	home, _ := homedir.Dir()
	var httpCacheDir = filepath.Join(home, ".kube", "http-cache")

	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(home, ".kube", "cache", "discovery"), p.RestConfig.Host)
	return diskcached.NewCachedDiscoveryClientForConfig(&p.RestConfig, discoveryCacheDir, httpCacheDir, 10*time.Minute)
}

func (p *KubeProvider) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, _ := p.ToDiscoveryClient()
	if discoveryClient != nil {
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
		expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
		return expander, nil
	}

	return nil, fmt.Errorf("no restmapper")
}

var kubectlApplyRetryCount uint64

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, diag.Diagnostics) {

	var cfg *restclient.Config
	var err error
	if d.Get("load_config_file").(bool) {
		// Config file loading
		cfg, err = tryLoadingConfigFile(d)
	}

	kubectlApplyRetryCount = uint64(d.Get("apply_retry_count").(int))
	if os.Getenv("KUBECTL_PROVIDER_APPLY_RETRY_COUNT") != "" {
		applyEnvValue, _ := strconv.Atoi(os.Getenv("KUBECTL_PROVIDER_APPLY_RETRY_COUNT"))
		kubectlApplyRetryCount = uint64(applyEnvValue)
	}

	if err != nil {
		return nil, diag.FromErr(err)
	}
	if cfg == nil {
		cfg = &restclient.Config{}
	}

	cfg.QPS = 100.0
	cfg.Burst = 100

	// Overriding with static configuration
	cfg.UserAgent = fmt.Sprintf("HashiCorp/1.0 Terraform/%s", terraformVersion)

	if v, ok := d.GetOk("host"); ok {
		cfg.Host = v.(string)
	}
	if v, ok := d.GetOk("username"); ok {
		cfg.Username = v.(string)
	}
	if v, ok := d.GetOk("password"); ok {
		cfg.Password = v.(string)
	}
	if v, ok := d.GetOk("insecure"); ok {
		cfg.Insecure = v.(bool)
	}
	if v, ok := d.GetOk("cluster_ca_certificate"); ok {
		cfg.CAData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("client_certificate"); ok {
		cfg.CertData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("client_key"); ok {
		cfg.KeyData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("token"); ok {
		cfg.BearerToken = v.(string)
	}

	if v, ok := d.GetOk("exec"); ok {
		exec := &clientcmdapi.ExecConfig{}
		if spec, ok := v.([]interface{})[0].(map[string]interface{}); ok {
			exec.APIVersion = spec["api_version"].(string)
			exec.Command = spec["command"].(string)
			exec.Args = expandStringSlice(spec["args"].([]interface{}))
			for kk, vv := range spec["env"].(map[string]interface{}) {
				exec.Env = append(exec.Env, clientcmdapi.ExecEnvVar{Name: kk, Value: vv.(string)})
			}
		} else {
			return nil, diag.FromErr(fmt.Errorf("failed to parse exec"))
		}
		cfg.ExecProvider = exec
	}

	k, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, diag.FromErr(fmt.Errorf("failed to configure: %s", err))
	}

	a, err := aggregator.NewForConfig(cfg)
	if err != nil {
		return nil, diag.FromErr(fmt.Errorf("failed to configure: %s", err))
	}

	// dereference config to create a shallow copy, allowing each func
	// to manipulate the state without affecting another func
	return &KubeProvider{
		MainClientset:       k,
		RestConfig:          *cfg,
		AggregatorClientset: a,
	}, nil
}

func tryLoadingConfigFile(d *schema.ResourceData) (*restclient.Config, error) {
	path, err := homedir.Expand(d.Get("config_path").(string))
	if err != nil {
		return nil, err
	}

	loader := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: path,
	}

	overrides := &clientcmd.ConfigOverrides{}
	ctxSuffix := "; default context"

	ctx, ctxOk := d.GetOk("config_context")
	authInfo, authInfoOk := d.GetOk("config_context_auth_info")
	cluster, clusterOk := d.GetOk("config_context_cluster")
	if ctxOk || authInfoOk || clusterOk {
		ctxSuffix = "; overriden context"
		if ctxOk {
			overrides.CurrentContext = ctx.(string)
			ctxSuffix += fmt.Sprintf("; config ctx: %s", overrides.CurrentContext)
			log.Printf("[DEBUG] Using custom current context: %q", overrides.CurrentContext)
		}

		overrides.Context = clientcmdapi.Context{}
		if authInfoOk {
			overrides.Context.AuthInfo = authInfo.(string)
			ctxSuffix += fmt.Sprintf("; auth_info: %s", overrides.Context.AuthInfo)
		}
		if clusterOk {
			overrides.Context.Cluster = cluster.(string)
			ctxSuffix += fmt.Sprintf("; cluster: %s", overrides.Context.Cluster)
		}
		log.Printf("[DEBUG] Using overidden context: %#v", overrides.Context)
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		if pathErr, ok := err.(*os.PathError); ok && os.IsNotExist(pathErr.Err) {
			log.Printf("[INFO] Unable to load config file as it doesn't exist at %q", path)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load config (%s%s): %s", path, ctxSuffix, err)
	}

	log.Printf("[INFO] Successfully loaded config file (%s%s)", path, ctxSuffix)
	return cfg, nil
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
