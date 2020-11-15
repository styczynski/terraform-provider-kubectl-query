package kubernetes

import (
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func dataSourceKubectlPodsRead(d *schema.ResourceData, meta interface{}) error {
	provider := meta.(*KubeProvider)
	clientConfig, err := provider.ToRESTConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	pods, err := client.CoreV1().Pods("default").List(v1.ListOptions{})
	if err != nil {
		return err
	}

	properties := map[string]interface{}{}
	pods_list := []map[string]interface{}{}
	for _, pod := range (*pods).Items {
		pods_list = append(pods_list, map[string]interface{}{
			"kind":            pod.Kind,
			"status":          pod.Status.String(),
			"labels":          pod.Labels,
			"cluster_name":     pod.ClusterName,
			"generate_name":    pod.GenerateName,
			"resource_version": pod.ResourceVersion,
			"annotations":     pod.Annotations,
			"uid":             string(pod.UID),
		})
	}

	properties["pods"] = pods_list

	for k, v := range properties {
		err := d.Set(k, v)
		if err != nil {
			return err
		}
	}

	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return nil
}

func dataSourceKubectlPodsSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"triggers": {
			Type:     schema.TypeMap,
			Optional: true,
			ForceNew: true,
		},
		"pods": &schema.Schema{
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
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
				},
			},
		},
	}
}

func resourceKubectlPods() *schema.Resource {
	return &schema.Resource{
		Create: dataSourceKubectlPodsRead,
		Read:   dataSourceKubectlPodsRead,
		Delete: dataSourceKubectlPodsDelete,
		Schema: dataSourceKubectlPodsSchema(),
	}
}

func dataSourceKubectlPods() *schema.Resource {
	return &schema.Resource{
		Read:   dataSourceKubectlPodsRead,
		Schema: dataSourceKubectlPodsSchema(),
	}
}

func dataSourceKubectlPodsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
