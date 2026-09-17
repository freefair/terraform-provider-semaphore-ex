package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewProjectIntegrationMatcherResource() resource.Resource {
	return &exRecordResource{spec: projectIntegrationMatcherSpec()}
}
