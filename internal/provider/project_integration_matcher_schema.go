package provider

import superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"

func ProjectIntegrationMatcherSchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{MarkdownDescription: "Manage or read a condition that an incoming integration request must match."},
		Attributes: map[string]superschema.Attribute{
			"id":             exRecordID("Matcher ID assigned by Semaphore EX."),
			"project_id":     exRecordParent("Project containing the integration."),
			"integration_id": exRecordParent("Integration containing the matcher."),
			"name":           exRecordString("Matcher name."),
			"match_type":     exRecordString("Request component to inspect: header or body.", "header", "body"),
			"method":         exRecordDefaultString("Comparison operator.", "equals", "equals", "unequals", "contains"),
			"body_data_type": exRecordDefaultString("Body interpretation for body matchers.", "json", "json", "string"),
			"key":            exRecordString("Header name or JSON field path to match."),
			"value":          exRecordString("Expected value."),
		},
	}
}

func projectIntegrationMatcherSpec() exRecordSpec {
	return exRecordSpec{
		name:             "project_integration_matcher",
		collection:       "/project/{project_id}/integrations/{integration_id}/matchers",
		item:             "/project/{project_id}/integrations/{integration_id}/matchers/{matcher_id}",
		idParameter:      "matcher_id",
		scopes:           map[string]string{"project_id": "project_id", "integration_id": "integration_id"},
		bodyFields:       []string{"integration_id", "name", "match_type", "method", "body_data_type", "key", "value"},
		importLabels:     []string{"project", "integration", "matcher"},
		importAttributes: []string{"project_id", "integration_id", "id"},
		schema:           ProjectIntegrationMatcherSchema(),
	}
}
