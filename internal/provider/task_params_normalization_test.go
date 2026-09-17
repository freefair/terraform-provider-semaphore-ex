package provider

import (
	"context"
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTaskParamsEmptyNormalization(t *testing.T) {
	ctx := context.Background()
	old := &TaskParamsModel{GitBranch: types.StringValue(""), Ansible: &AnsibleTaskParamsModel{Tags: types.ListValueMust(types.StringType, nil)}, Terraform: &TerraformTaskParamsModel{}}
	result := convertTaskPramsToTaskParamsModel(ctx, &models.TaskPrams{}, old)
	require.NotNil(t, result)
	require.NotNil(t, result.Ansible)
	require.NotNil(t, result.Terraform)
	require.Equal(t, types.StringValue(""), result.GitBranch)
	require.False(t, result.Ansible.Tags.IsNull())
	require.Empty(t, result.Ansible.Tags.Elements())
	require.Nil(t, convertTaskPramsToTaskParamsModel(ctx, nil, &TaskParamsModel{GitBranch: types.StringValue("main")}))
	require.NotNil(t, convertTaskPramsToTaskParamsModel(ctx, nil, old))
}

func TestAcc_TaskParamsEmptyNormalization(t *testing.T) {
	for _, test := range []struct {
		name     string
		config   func(string, string) string
		resource string
	}{
		{"template", testAccProjectTemplateConfig, "semaphore_ex_project_template.test"},
		{"integration", testAccProjectIntegrationConfig, "semaphore_ex_project_integration.test"},
		{"schedule", func(suffix, extra string) string {
			return testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_project_schedule" "test" {
 project_id = semaphore_ex_project.test.id
 template_id = semaphore_ex_project_template.test.id
 name = "Empty defaults"
 cron_format = "0 0 * * *"
 enabled = false
 %s
}
`, extra)
		}, "semaphore_ex_project_schedule.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			suffix := acctest.RandString(8)
			extra := `task_params = { git_branch = "", arguments = "", ansible = { tags = [] }, terraform = { reconfigure = false } }`
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
				{Config: test.config(suffix, extra), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(test.resource, "task_params.git_branch", ""), resource.TestCheckResourceAttr(test.resource, "task_params.ansible.tags.#", "0"), resource.TestCheckResourceAttr(test.resource, "task_params.terraform.reconfigure", "false"))},
				{Config: test.config(suffix, extra), PlanOnly: true},
				{Config: test.config(suffix, ""), Check: resource.TestCheckNoResourceAttr(test.resource, "task_params.git_branch")},
			}})
		})
	}
}
