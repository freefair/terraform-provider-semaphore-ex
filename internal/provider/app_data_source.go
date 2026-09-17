package provider

import "github.com/hashicorp/terraform-plugin-framework/datasource"

func NewAppDataSource() datasource.DataSource { return &exRecordDataSource{spec: appSpec()} }
