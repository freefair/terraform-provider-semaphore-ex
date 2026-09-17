package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const exDockerPolicyResponse = `{"revision":3,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","allowed_images":["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"require_digest":true,"allowed_networks":["none"],"network":"none","user":"65534:0","nano_cpus":1000000000,"memory_bytes":536870912,"pids_limit":256,"pull_timeout_seconds":300,"max_image_size_bytes":2147483648,"seccomp_profile":"default","apparmor_profile":"docker-default","allow_privileged":false,"allow_bind_mounts":false,"allow_devices":false,"allow_host_namespaces":false}`
const exKubernetesPolicyResponse = `{"cluster_alias":"qa-cluster","revision":4,"hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","allowed_namespaces":["semaphore-jobs"],"allowed_images":["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"allowed_service_accounts":["semaphore-task"],"allowed_runtime_classes":[""],"runtime_class":"","allowed_volume_types":["emptyDir","secret"],"allowed_network_profiles":["deny-all"],"network_profile":"deny-all","network_policy_enforcement":"network-policy","resources":{"cpu_request_milli":100,"cpu_limit_milli":500,"memory_request_bytes":67108864,"memory_limit_bytes":268435456,"ephemeral_storage_request_bytes":67108864,"ephemeral_storage_limit_bytes":268435456},"terminal_retention_seconds":3600}`

func TestEXDockerExecutionPolicyWriteUsesStateRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/runners/docker-policy" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"revision":3`) || strings.Contains(string(body), `"hash"`) {
			t.Fatalf("payload = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(exDockerPolicyResponse))
	}))
	defer server.Close()
	current, err := exDockerExecutionPolicyModelFromWire(context.Background(), exExecutorPolicyTestJSONMap(t, exDockerPolicyResponse))
	if err != nil {
		t.Fatal(err)
	}
	next, err := exWriteDockerExecutionPolicy(context.Background(), newEXTestClient(t, server.URL), current)
	if err != nil || next.Revision.ValueInt64() != 3 || next.ID.ValueString() != "docker" {
		t.Fatalf("state = %#v, error = %v", next, err)
	}
}

func exExecutorPolicyTestJSONMap(t *testing.T, payload string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func exDockerExecutionPolicyConfig(pidsLimit int64) string {
	return `resource "semaphore_ex_docker_execution_policy" "test" {
  allowed_images = ["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
  require_digest = true
  allowed_networks = ["none"]
  network = "none"
  user = "65534:0"
  nano_cpus = 1000000000
  memory_bytes = 536870912
  pids_limit = ` + fmt.Sprintf("%d", pidsLimit) + `
  pull_timeout_seconds = 300
  max_image_size_bytes = 2147483648
  seccomp_profile = "default"
  apparmor_profile = "docker-default"
  allow_privileged = false
  allow_bind_mounts = false
  allow_devices = false
  allow_host_namespaces = false
}`
}

func exKubernetesExecutionPolicyConfig() string {
	return `resource "semaphore_ex_kubernetes_execution_policy" "test" {
  cluster_alias = "acceptance-cluster"
  allowed_namespaces = ["semaphore-jobs"]
  allowed_images = ["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
  allowed_service_accounts = ["semaphore-task"]
  allowed_runtime_classes = [""]
  runtime_class = ""
  allowed_volume_types = ["emptyDir", "secret"]
  allowed_network_profiles = ["deny-all"]
  network_profile = "deny-all"
  network_policy_enforcement = "network-policy"
  resources = {
    cpu_request_milli = 100
    cpu_limit_milli = 500
    memory_request_bytes = 67108864
    memory_limit_bytes = 268435456
    ephemeral_storage_request_bytes = 67108864
    ephemeral_storage_limit_bytes = 268435456
  }
  terminal_retention_seconds = 3600
}`
}

func TestAcc_EXDockerExecutionPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: exDockerExecutionPolicyConfig(256), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_docker_execution_policy.test", "network", "none"), resource.TestCheckResourceAttrSet("semaphore_ex_docker_execution_policy.test", "revision"))},
		{ResourceName: "semaphore_ex_docker_execution_policy.test", ImportState: true, ImportStateVerify: true},
		{Config: exDockerExecutionPolicyConfig(128), Check: resource.TestCheckResourceAttr("semaphore_ex_docker_execution_policy.test", "pids_limit", "128")},
	}})
}

func TestAcc_EXKubernetesExecutionPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: exKubernetesExecutionPolicyConfig(), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_kubernetes_execution_policy.test", "cluster_alias", "acceptance-cluster"), resource.TestCheckResourceAttrSet("semaphore_ex_kubernetes_execution_policy.test", "revision"))},
		{ResourceName: "semaphore_ex_kubernetes_execution_policy.test", ImportState: true, ImportStateVerify: true},
	}})
}

func TestEXKubernetesExecutionPolicyDefaultResetUsesStateRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/runners/kubernetes-policies/qa-cluster" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		payload := string(body)
		for _, wanted := range []string{`"cluster_alias":"qa-cluster"`, `"revision":4`, `"allowed_images":[]`, `"network_policy_enforcement":"unsupported"`} {
			if !strings.Contains(payload, wanted) {
				t.Fatalf("payload lacks %s: %s", wanted, payload)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(exKubernetesPolicyResponse))
	}))
	defer server.Close()
	model := exDefaultKubernetesExecutionPolicy(types.StringValue("qa-cluster"), types.Int64Value(4))
	next, err := exWriteKubernetesExecutionPolicy(context.Background(), newEXTestClient(t, server.URL), model)
	if err != nil || next.Revision.ValueInt64() != 4 || next.ClusterAlias.ValueString() != "qa-cluster" {
		t.Fatalf("state = %#v, error = %v", next, err)
	}
}
