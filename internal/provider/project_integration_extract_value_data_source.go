package provider

import "github.com/hashicorp/terraform-plugin-framework/datasource"

func NewProjectIntegrationExtractValueDataSource() datasource.DataSource {
	return &exRecordDataSource{spec: projectIntegrationExtractValueSpec()}
}
