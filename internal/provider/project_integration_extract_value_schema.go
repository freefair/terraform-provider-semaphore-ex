package provider

import superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"

func ProjectIntegrationExtractValueSchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{MarkdownDescription: "Manage or read a value extracted from incoming integration requests into task parameters or environment variables."},
		Attributes: map[string]superschema.Attribute{
			"id":             exRecordID("Extracted-value rule ID assigned by Semaphore EX."),
			"project_id":     exRecordParent("Project containing the integration."),
			"integration_id": exRecordParent("Integration containing the rule."),
			"name":           exRecordString("Rule name."),
			"value_source":   exRecordString("Request component to extract from.", "header", "body"),
			"body_data_type": exRecordDefaultString("Interpretation of the request body.", "json", "json", "string"),
			"key":            exRecordDefaultString("Header name or JSON field path. Empty only when extracting the entire string body.", ""),
			"variable":       exRecordString("Destination environment variable or task parameter name."),
			"variable_type":  exRecordString("Destination type.", "environment", "task"),
		},
	}
}

func projectIntegrationExtractValueSpec() exRecordSpec {
	return exRecordSpec{
		name:             "project_integration_extract_value",
		collection:       "/project/{project_id}/integrations/{integration_id}/values",
		item:             "/project/{project_id}/integrations/{integration_id}/values/{value_id}",
		idParameter:      "value_id",
		scopes:           map[string]string{"project_id": "project_id", "integration_id": "integration_id"},
		bodyFields:       []string{"integration_id", "name", "value_source", "body_data_type", "key", "variable", "variable_type"},
		importLabels:     []string{"project", "integration", "value"},
		importAttributes: []string{"project_id", "integration_id", "id"},
		schema:           ProjectIntegrationExtractValueSchema(),
	}
}
