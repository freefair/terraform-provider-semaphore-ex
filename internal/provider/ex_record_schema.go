package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

func exRecordID(description string) superschema.Int64Attribute {
	return superschema.Int64Attribute{
		Common:     &schemaR.Int64Attribute{MarkdownDescription: description},
		Resource:   &schemaR.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		DataSource: &schemaD.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
	}
}

func exRecordParent(description string) superschema.Int64Attribute {
	return superschema.Int64Attribute{
		Common:   &schemaR.Int64Attribute{MarkdownDescription: description, Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		Resource: &schemaR.Int64Attribute{PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
	}
}

func exRecordString(description string, choices ...string) superschema.StringAttribute {
	validators := []validator.String{stringvalidator.LengthAtLeast(1)}
	if len(choices) > 0 {
		validators = append(validators, stringvalidator.OneOf(choices...))
	}
	return superschema.StringAttribute{
		Common:     &schemaR.StringAttribute{MarkdownDescription: description},
		Resource:   &schemaR.StringAttribute{Required: true, Validators: validators},
		DataSource: &schemaD.StringAttribute{Computed: true},
	}
}

func exRecordDefaultString(description, value string, choices ...string) superschema.StringAttribute {
	attribute := exRecordString(description, choices...)
	attribute.Resource.Required = false
	attribute.Resource.Optional = true
	attribute.Resource.Computed = true
	attribute.Resource.Default = stringdefault.StaticString(value)
	if value == "" {
		attribute.Resource.Validators = nil
	}
	return attribute
}
