package provider

import (
	"context"
	"encoding/json"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTemplatePreservesUnconfiguredEXSettings(t *testing.T) {
	var input models.Template
	require.NoError(t, json.Unmarshal([]byte(`{"id":1,"project_id":1,"repository_id":1,"inventory_id":1,"environment_ids":[],"name":"existing","app":"bash","playbook":"run.sh","allow_parallel_tasks":true,"allow_override_branch_in_task":true,"jwt_params":{"enabled":true,"audience":["service"],"ttl":"10m"}}`), &input))
	state := convertTemplateResponseToProjectTemplateModel(context.Background(), &input, &ProjectTemplateModel{SurveyVars: types.ListNull(ProjectTemplateSurveyVarType), Vaults: types.ListNull(ProjectTemplateVaultType)})
	request := convertProjectTemplateModelToTemplateRequest(context.Background(), state)
	body, err := json.Marshal(request)
	require.NoError(t, err)
	var output map[string]any
	require.NoError(t, json.Unmarshal(body, &output))
	require.Equal(t, true, output["allow_parallel_tasks"])
	require.Equal(t, true, output["allow_override_branch_in_task"])
	require.Equal(t, map[string]any{"enabled": true, "audience": []any{"service"}, "ttl": "10m"}, output["jwt_params"])
}
