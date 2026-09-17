package provider

import "github.com/hashicorp/terraform-plugin-framework/datasource"

func NewProjectTerraformBackendAliasDataSource() datasource.DataSource {
	return &exRecordDataSource{spec: projectTerraformBackendAliasSpec()}
}
