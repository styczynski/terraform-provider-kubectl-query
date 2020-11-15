package kubernetes

import (
	"crypto/sha256"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func dataSourceKubectlServicesRead(d *schema.ResourceData, meta interface{}) error {
	provider := meta.(*KubeProvider)
	clientConfig, err := provider.ToRESTConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	services, err := client.CoreV1().Services("default").List(v1.ListOptions{})
	if err != nil {
		return err
	}

	properties := map[string]interface{}{}
	servicesList := []interface{}{}
	for _, service := range (*services).Items {
		serviceProperties := map[string]interface{}{
			"type":            string(service.Spec.Type),
			"kind":            service.Kind,
			"status":          service.Status.String(),
			"labels":          service.Labels,
			"cluster_name":     service.ClusterName,
			"generate_name":    service.GenerateName,
			"resource_version": service.ResourceVersion,
			"annotations":     service.Annotations,
			"uid":             service.UID,
			"external_ips":    service.Spec.ExternalIPs,
			"load_balancer_ip": service.Spec.LoadBalancerIP,
			"external_traffic_policy": string(service.Spec.ExternalTrafficPolicy),
			"external_name": service.Spec.ExternalName,
			"load_balancer_source_ranges": service.Spec.LoadBalancerSourceRanges,
		}
		serviceIngress := []map[string]interface{}{}
		servicesIngressPrefixes := []string{}
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			serviceIngress = append(serviceIngress, map[string]interface{}{
				"ip": ingress.IP,
				"hostname": ingress.Hostname,
			})
			if len(ingress.IP) > 0 {
				servicesIngressPrefixes = append(servicesIngressPrefixes, ingress.IP)
			} else if len(ingress.Hostname) > 0 {
				servicesIngressPrefixes = append(servicesIngressPrefixes, ingress.Hostname)
			}
		}
		serviceProperties["ingress"] = serviceIngress

		serviceExternalAdresses := []string{}
		servicePorts := []map[string]interface{}{}
		for _, portSpec := range service.Spec.Ports {
			port := int(portSpec.Port)
			servicePorts = append(servicePorts, map[string]interface{}{
				"port": port,
				"node_port": int(portSpec.NodePort),
				"protocol": string(portSpec.Protocol),
				"name": portSpec.Name,
			})
			for _, prefix := range servicesIngressPrefixes {
				serviceExternalAdresses = append(serviceExternalAdresses, fmt.Sprintf("%s:%d", prefix, port))
			}
		}
		serviceProperties["ports"] = servicePorts
		serviceProperties["external_addresses"] = serviceExternalAdresses


		servicesList = append(servicesList, serviceProperties)
	}

	properties["services"] = servicesList

	for k, v := range properties {
		err := d.Set(k, v)
		if err != nil {
			return err
		}
	}

	props, err := yaml.Marshal(properties)
	if err != nil {
		return err
	}
	d.SetId(fmt.Sprintf("%x", sha256.Sum256(props)))

	return nil
}

func dataSourceKubectlServicesSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"triggers": {
			Type:     schema.TypeMap,
			Optional: true,
			ForceNew: true,
		},
		"services": &schema.Schema{
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"type": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"kind": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"status": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"labels": &schema.Schema{
						Type:     schema.TypeMap,
						Computed: true,
					},
					"cluster_name": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"generate_name": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"resource_version": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"annotations": &schema.Schema{
						Type:     schema.TypeMap,
						Computed: true,
					},
					"uid": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"load_balancer_ip": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"ingress": &schema.Schema{
						Type:     schema.TypeList,
						Computed: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"ip": &schema.Schema{
									Type:     schema.TypeString,
									Computed: true,
								},
								"hostname": &schema.Schema{
									Type:     schema.TypeString,
									Computed: true,
								},
							},
						},
					},
					"external_traffic_policy": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"external_ips": &schema.Schema{
						Type:     schema.TypeList,
						Computed: true,
						Elem: schema.TypeString,
					},
					"external_name": &schema.Schema{
						Type:     schema.TypeString,
						Computed: true,
					},
					"load_balancer_source_ranges": &schema.Schema{
						Type:     schema.TypeList,
						Computed: true,
						Elem: schema.TypeString,
					},
					"external_addresses": &schema.Schema{
						Type:     schema.TypeList,
						Computed: true,
						Elem: schema.TypeString,
					},
					"ports": &schema.Schema{
						Type:     schema.TypeList,
						Computed: true,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"port": &schema.Schema{
									Type:     schema.TypeInt,
									Computed: true,
								},
								"node_port": &schema.Schema{
									Type:     schema.TypeInt,
									Computed: true,
								},
								"protocol": &schema.Schema{
									Type:     schema.TypeString,
									Computed: true,
								},
								"name": &schema.Schema{
									Type:     schema.TypeString,
									Computed: true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceKubectlServices() *schema.Resource {
	return &schema.Resource{
		Create: dataSourceKubectlServicesRead,
		Read:   dataSourceKubectlServicesRead,
		Delete: dataSourceKubectlServicesDelete,
		Schema: dataSourceKubectlServicesSchema(),
	}
}

func dataSourceKubectlServices() *schema.Resource {
	return &schema.Resource{
		Read:   dataSourceKubectlServicesRead,
		Schema: dataSourceKubectlServicesSchema(),
	}
}

func dataSourceKubectlServicesDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
