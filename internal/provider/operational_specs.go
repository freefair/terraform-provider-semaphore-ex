package provider

import (
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	as "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type operationalSpec struct {
	name, description, method, route, bodyRoot, scopeMode string
	inputs                                                map[string]as.Attribute
	paths, query                                          map[string]string
	body                                                  []string
	preview, token                                        bool
}

func operationString(description string) as.StringAttribute {
	return as.StringAttribute{Required: true, MarkdownDescription: description, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}
}
func operationID() as.Int64Attribute {
	return as.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}
}
func operationObject(required bool, description string) as.DynamicAttribute {
	return as.DynamicAttribute{Required: required, Optional: !required, MarkdownDescription: description}
}
func operationalSpecs() map[string]operationalSpec {
	specs := map[string]operationalSpec{}
	add := func(spec operationalSpec) { specs[spec.name] = spec }
	for _, name := range []string{"ldap_group_preview", "ldap_group_apply", "ldap_group_reconcile", "oidc_group_preview"} {
		inputs := map[string]as.Attribute{"provider_id": operationString("Configured identity provider ID.")}
		body := []string{"provider_id"}
		kind, op := "ldap", "preview"
		switch name {
		case "ldap_group_apply":
			op = "apply"
			inputs["preview_token"] = as.StringAttribute{Required: true, WriteOnly: true, MarkdownDescription: "Fresh preview token from the same LDAP provider; the server rechecks mapping and directory revisions."}
			body = append(body, "preview_token")
		case "ldap_group_reconcile":
			op = "reconcile"
		case "oidc_group_preview":
			kind = "oidc"
			inputs["user_id"] = operationID()
			inputs["claim"] = as.DynamicAttribute{Required: true, WriteOnly: true, MarkdownDescription: "Native group claim value matching the provider configuration."}
			body = append(body, "user_id", "claim")
		}
		description := "Records a fresh identity-mapping preview. It does not apply assignments. Use the matching ephemeral resource to consume the structured result and token without persistent state."
		if op == "apply" {
			description = "Applies a fresh LDAP mapping preview explicitly. This changes role assignments; stale previews fail without retry."
		}
		if op == "reconcile" {
			description = "Reconciles LDAP group mappings immediately and changes role assignments. This is not a dry run."
		}
		add(operationalSpec{name: name, description: description, method: http.MethodPost, route: "/capabilities/" + kind + "/group-mappings/" + op, inputs: inputs, body: body, preview: op == "preview", token: op == "preview"})
	}
	for _, project := range []bool{false, true} {
		scope, prefix := "global", ""
		if project {
			scope = "project"
			prefix = "/project/{project_id}"
		}
		scoped := func() (map[string]as.Attribute, map[string]string) {
			inputs := map[string]as.Attribute{}
			paths := map[string]string{}
			if project {
				inputs["project_id"] = operationID()
				paths["project_id"] = "project_id"
			}
			return inputs, paths
		}
		for _, op := range []string{"destination_test", "delivery_retry", "routing_preview"} {
			inputs, paths := scoped()
			route := prefix + "/notification-governance/"
			description := "Evaluates notification routing without sending a notification."
			bodyRoot := ""
			switch op {
			case "destination_test":
				inputs["destination_id"] = operationID()
				paths["destination_id"] = "destination_id"
				route += "destinations/{destination_id}/test"
				description = "Queues a real test notification for the selected destination. The dispatcher may deliver it asynchronously."
			case "delivery_retry":
				inputs["delivery_id"] = operationID()
				paths["delivery_id"] = "delivery_id"
				route += "deliveries/{delivery_id}/retry"
				description = "Retries an existing notification delivery explicitly. The server checks current destination revision and claim state."
			case "routing_preview":
				route += "routing/preview"
				inputs["event"] = operationObject(true, "Native NotificationEvent object. Scope and project identity are derived from this operation and cannot be changed by the event.")
				bodyRoot = "event"
			}
			add(operationalSpec{name: scope + "_notification_" + op, description: description, method: http.MethodPost, route: route, inputs: inputs, paths: paths, bodyRoot: bodyRoot, preview: op == "routing_preview", scopeMode: func() string {
				if op == "routing_preview" {
					return "notification"
				}
				return ""
			}()})
		}
		for _, op := range []string{"validate", "diff", "test", "impact", "rollback"} {
			inputs, paths := scoped()
			spec := operationalSpec{name: scope + "_policy_guardrail_" + op, description: "Evaluates or compares policy guardrails without publishing or saving a policy.", method: http.MethodPost, route: prefix + "/policy-guardrails/" + op, inputs: inputs, paths: paths, preview: op != "rollback"}
			switch op {
			case "validate":
				inputs["source_yaml"] = operationString("Authored policy YAML to validate; yamlencode supports native HCL authoring.")
				spec.body = []string{"source_yaml"}
			case "diff":
				spec.method = http.MethodGet
				inputs["from_revision"] = operationID()
				inputs["to_revision"] = operationID()
				spec.query = map[string]string{"from_revision": "from_revision", "to_revision": "to_revision"}
			case "test":
				inputs["source_yaml"] = operationString("Authored YAML to evaluate against the supplied fixture; this endpoint does not load active policy.")
				inputs["input"] = operationObject(true, "Value-free PolicyGuardrailEvaluationInput: project_id, intent, evaluated_at, template/workflow, runner, executor and credential-reference metadata. No credentials or executable payloads.")
				spec.body = []string{"source_yaml", "input"}
				spec.scopeMode = "guardrail_input"
			case "impact":
				inputs["inputs"] = operationObject(true, "Native list of 1–100 value-free PolicyGuardrailEvaluationInput objects.")
				spec.body = []string{"inputs"}
				spec.scopeMode = "guardrail_inputs"
			case "rollback":
				inputs["revision"] = operationID()
				inputs["expected_draft_revision"] = operationID()
				inputs["reason"] = operationString("Explicit rollback reason; the server rejects blank, multiline or oversized reasons.")
				spec.body = []string{"revision", "expected_draft_revision", "reason"}
				spec.description = "Publishes a new policy revision from a prior revision. Requires an explicit reason and the exact current draft revision; conflicts are not retried."
			}
			add(spec)
		}
	}
	add(operationalSpec{name: "audit_webhook_test", description: "Creates a test audit delivery and attempts real outbound HTTP delivery using the selected signing key.", method: http.MethodPost, route: "/audit-webhook/test", inputs: map[string]as.Attribute{"key": as.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("current", "next")}, MarkdownDescription: "Signing slot; omission uses current."}}, query: map[string]string{"key": "key"}})
	add(operationalSpec{name: "docker_execution_policy_test", description: "Evaluates a Docker execution request against stored policy. No container is created.", method: http.MethodPost, route: "/runners/docker-policy/test", inputs: map[string]as.Attribute{"request": dockerTestRequestAttribute()}, bodyRoot: "request", preview: true})
	add(operationalSpec{name: "kubernetes_execution_policy_test", description: "Evaluates a Kubernetes manifest summary against stored policy. No Job or other Kubernetes object is created.", method: http.MethodPost, route: "/runners/kubernetes-policies/{cluster_alias}/test", inputs: map[string]as.Attribute{"cluster_alias": operationString("Stored cluster policy alias."), "request": kubernetesTestRequestAttribute()}, paths: map[string]string{"cluster_alias": "cluster_alias"}, bodyRoot: "request", scopeMode: "kubernetes", preview: true})
	add(operationalSpec{name: "project_deployment_window_preview", description: "Evaluates a proposed deployment-window policy for exactly one template or workflow without storing a decision or override.", method: http.MethodPost, route: "/project/{project_id}/deployment-windows/preview", inputs: map[string]as.Attribute{"project_id": operationID(), "policy": operationObject(true, "Policy object containing revision, timezone, default, rules and exactly one template_id or workflow_id. Project scope comes from project_id.")}, paths: map[string]string{"project_id": "project_id"}, bodyRoot: "policy", scopeMode: "deployment", preview: true})
	for _, op := range []string{"confirm", "reject", "retry_recovery"} {
		route := op
		if op == "retry_recovery" {
			route = "retry-recovery"
		}
		add(operationalSpec{name: "project_task_" + op, description: "Explicit task control. Confirm/reject require an active task runner; recovery retry requires an eligible persisted task and recovery manager.", method: http.MethodPost, route: "/project/{project_id}/tasks/{task_id}/" + route, inputs: map[string]as.Attribute{"project_id": operationID(), "task_id": operationID()}, paths: map[string]string{"project_id": "project_id", "task_id": "task_id"}})
	}
	add(operationalSpec{name: "project_workflow_retry_reconcile", description: "Retries workflow reconciliation and clears its retry/quarantine state. Finished runs remain unchanged.", method: http.MethodPost, route: "/project/{project_id}/workflows/{workflow_id}/runs/{run_id}/retry-reconcile", inputs: map[string]as.Attribute{"project_id": operationID(), "workflow_id": operationID(), "run_id": operationID()}, paths: map[string]string{"project_id": "project_id", "workflow_id": "workflow_id", "run_id": "run_id"}})
	add(operationalSpec{name: "workflow_trigger_test", description: "Fires a real workflow trigger and creates an invocation/run. This is an operational test, not a dry-run preview.", method: http.MethodPost, route: "/project/{project_id}/workflows/{workflow_id}/triggers/{trigger_id}/test", inputs: map[string]as.Attribute{"project_id": operationID(), "workflow_id": operationID(), "trigger_id": operationID(), "inputs": operationObject(false, "Optional native object of declared trigger inputs.")}, paths: map[string]string{"project_id": "project_id", "workflow_id": "workflow_id", "trigger_id": "trigger_id"}, body: []string{"inputs"}})
	return specs
}
func dockerTestRequestAttribute() as.SingleNestedAttribute {
	fields := map[string]as.Attribute{}
	for _, name := range []string{"image", "network", "user"} {
		fields[name] = as.StringAttribute{Optional: true}
	}
	for _, name := range []string{"nano_cpus", "memory_bytes", "pids_limit"} {
		fields[name] = as.Int64Attribute{Optional: true}
	}
	for _, name := range []string{"privileged", "has_bind_mounts", "has_devices", "has_host_namespaces", "has_capabilities", "read_only_rootfs"} {
		fields[name] = as.BoolAttribute{Optional: true}
	}
	return as.SingleNestedAttribute{Required: true, Attributes: fields}
}
func kubernetesTestRequestAttribute() as.SingleNestedAttribute {
	fields := map[string]as.Attribute{}
	for _, name := range []string{"namespace", "task_image", "helper_image", "service_account", "runtime_class", "network_profile"} {
		fields[name] = as.StringAttribute{Optional: true}
	}
	fields["terminal_retention_seconds"] = as.Int64Attribute{Optional: true}
	fields["volume_types"] = as.ListAttribute{Optional: true, ElementType: types.StringType}
	resources := map[string]as.Attribute{}
	for name := range exKubernetesExecutionResourcesType {
		resources[name] = as.Int64Attribute{Optional: true}
	}
	fields["resources"] = as.SingleNestedAttribute{Optional: true, Attributes: resources}
	for _, name := range []string{"has_host_network", "has_host_pid", "has_host_ipc", "has_host_path", "has_projected_service_token", "has_privileged", "has_project_override", "has_restricted_security_profile"} {
		fields[name] = as.BoolAttribute{Optional: true}
	}
	return as.SingleNestedAttribute{Required: true, Attributes: fields}
}
func operationalNames() []string {
	names := []string{}
	for name := range operationalSpecs() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func operationalActions() []func() action.Action {
	specs := operationalSpecs()
	factories := []func() action.Action{}
	for _, name := range operationalNames() {
		spec := specs[name]
		factories = append(factories, func() action.Action { return &operationalAction{spec: spec} })
	}
	return factories
}
func operationalEphemeralResources() []func() ephemeral.EphemeralResource {
	specs := operationalSpecs()
	factories := []func() ephemeral.EphemeralResource{}
	for _, name := range operationalNames() {
		spec := specs[name]
		if spec.preview {
			factories = append(factories, func() ephemeral.EphemeralResource { return &operationalPreview{spec: spec} })
		}
	}
	return factories
}
