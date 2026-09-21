package provider

import (
	"context"
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSurveyChoicesPreserveOrderAndDuplicateLabels(t *testing.T) {
	model := convertTemplateResponseToProjectTemplateModel(context.Background(), &models.Template{SurveyVars: []*models.TemplateSurveyVar{{Name: "target", Type: "enum", Values: []*models.TemplateSurveyVarValue{{Name: "same", Value: "z"}, {Name: "same", Value: "a"}}}}}, &ProjectTemplateModel{})
	node := model.SurveyVars.Elements()[0].(types.Object)
	require.Contains(t, node.Attributes(), "choices")
	choices := node.Attributes()["choices"].(types.List)
	require.Len(t, choices.Elements(), 2)
	assert.Equal(t, types.StringValue("z"), choices.Elements()[0].(types.Object).Attributes()["value"])
	assert.Equal(t, types.StringValue("a"), choices.Elements()[1].(types.Object).Attributes()["value"])
	assert.True(t, node.Attributes()["enum_values"].IsNull(), "an unordered map cannot represent repeated labels")
}

func TestAcc_TemplateOrderedSurveyChoices(t *testing.T) {
	suffix := acctest.RandString(8)
	const address = "semaphore_ex_project_template.test"
	configuration := func(choices string) string {
		return testAccProjectTemplateConfig(suffix, fmt.Sprintf(`survey_vars = [{name="target",title="Target",type="enum",choices=%s}]`, choices))
	}
	initial := configuration(`[{name="same",value="z"},{name="same",value="a"}]`)
	changed := configuration(`[{name="first",value="one"},{name="second",value="two"}]`)
	cleared := configuration(`[]`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "survey_vars.0.choices.0.value", "z"), resource.TestCheckResourceAttr(address, "survey_vars.0.choices.1.value", "a"), resource.TestCheckNoResourceAttr(address, "survey_vars.0.enum_values"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID(address)},
		{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: changed, Check: resource.TestCheckResourceAttr(address, "survey_vars.0.enum_values.first", "one")},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID(address)},
		{Config: changed, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: cleared, Check: resource.TestCheckResourceAttr(address, "survey_vars.0.choices.#", "0")},
		{Config: cleared, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
