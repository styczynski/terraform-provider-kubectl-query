# Kubernetes "kubectl-query" Provider 

The query provider can be used to check the state and specs of the services/pods etc. in the k8s cluster.
Why anyone would need that?

Sometimes you deploy your services via [kubectl provider plugin](https://github.com/gavinbunney/terraform-provider-kubectl) and you have no idea what the endpoint of the load balancer is!
This plugin solves that problem and allows you to query services/pods etc. using kubectl.

## Installation

The provider can be installed and managed automatically by Terraform. Sample `versions.tf` file :

```hcl
terraform {
  required_version = ">= 0.13"

  required_providers {
    kubectl = {
      source  = "styczynski/kubectl-query"
      version = ">= 1.7.0"
    }
  }
}
```

## Quick Start

```hcl
provider "kubectl-query" {
  host                   = var.eks_cluster_endpoint
  cluster_ca_certificate = base64decode(var.eks_cluster_ca)
  token                  = data.aws_eks_cluster_auth.main.token
  load_config_file       = false
}

data "kubectl-query_services" "cluster_lbs" {}

output "cluster_lb_endpoint" {
    value = [for service in data.kubectl-query_services.cluster_lbs.services: service if service.type == "LoadBalancer"]
}
```

See [User Guide](https://registry.terraform.io/providers/styczynski/kubectl-query/latest) for details on installation and all the provided data and resource types.

---

### Inspiration

This plugin was written thanks to awesome [kubectl provider plugin](https://github.com/gavinbunney/terraform-provider-kubectl) by [gavinbunney](https://github.com/gavinbunney)