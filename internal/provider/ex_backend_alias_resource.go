package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewProjectTerraformBackendAliasResource() resource.Resource {
	return &exRecordResource{spec: projectTerraformBackendAliasSpec()}
}
