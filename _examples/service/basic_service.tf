provider "kubectl_query" {}

data "kubectl_query_services" "current" {}

output "services" {
    value = module.kubectl_query_services
}
