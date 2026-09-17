package provider

import "github.com/hashicorp/terraform-plugin-framework/datasource"

func NewProjectIntegrationMatcherDataSource() datasource.DataSource {
	return &exRecordDataSource{spec: projectIntegrationMatcherSpec()}
}
