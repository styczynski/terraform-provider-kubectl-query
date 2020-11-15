module github.com/gavinbunney/terraform-provider-kubectl

go 1.14

require (
	github.com/Azure/go-autorest/autorest v0.9.2 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.8.1-0.20191028180845-3492b2aff503 // indirect
	github.com/aws/aws-sdk-go v1.30.12 // indirect
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/hcl/v2 v2.6.0 // indirect
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.0.4
	github.com/icza/dyno v0.0.0-20180601094105-0c96289f9585
	github.com/mitchellh/go-homedir v1.1.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.12
	k8s.io/apimachinery v0.17.12
	k8s.io/cli-runtime v0.17.12
	k8s.io/client-go v0.17.12
	k8s.io/kube-aggregator v0.17.12
	k8s.io/kubectl v0.17.12
	sigs.k8s.io/yaml v1.1.0
)

replace github.com/Azure/go-autorest v10.15.4+incompatible => github.com/Azure/go-autorest v13.0.0+incompatible
