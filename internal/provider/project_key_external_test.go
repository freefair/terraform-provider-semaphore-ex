package provider

import (
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConvertProjectKeyRemoteReferenceRoundTrip(t *testing.T) {
	storageType := "vault"
	storageID := int64(7)
	remotePath := "service/token"
	key := &models.AccessKey{
		ID:                   3,
		ProjectID:            2,
		Name:                 "runtime token",
		Type:                 ProjectKeyTypeString,
		SourceStorageType:    &storageType,
		SourceStorageID:      &storageID,
		SourceStorageKey:     &remotePath,
		SourceStorageMount:   "team",
		SourceStorageVersion: 4,
		SourceStorageField:   "value",
	}

	model := convertAccessKeyResponseToProjectKeyModel(key, &ProjectKeyModel{String: &ProjectKeyString{Value: types.StringValue("")}})
	if model.String == nil || model.RemoteReference == nil || model.RemoteReference.Path.ValueString() != remotePath || model.RemoteReference.StorageID.ValueInt64() != storageID {
		t.Fatal("remote string key was not represented in Terraform state")
	}

	request := convertProjectKeyModelToAccessKeyRequest(model, resolvedSecrets{})
	if request.Type != ProjectKeyTypeString || request.SourceStorageType == nil || *request.SourceStorageType != storageType || request.SourceStorageKey == nil || *request.SourceStorageKey != remotePath {
		t.Fatal("remote string key was not represented in API request")
	}
}

func TestConvertProjectKeyStringWriteOnlyValue(t *testing.T) {
	plan := ProjectKeyModel{String: &ProjectKeyString{Value: types.StringNull()}}
	config := ProjectKeyModel{String: &ProjectKeyString{ValueWO: types.StringValue("ephemeral")}}
	request := convertProjectKeyModelToAccessKeyRequest(plan, resolveSecrets(&plan, &config))
	if request.Type != ProjectKeyTypeString || request.String != "ephemeral" {
		t.Fatal("write-only string value was not sent to the API")
	}
}
