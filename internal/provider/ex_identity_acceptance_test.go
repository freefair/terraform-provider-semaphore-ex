package provider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
)

func TestAcc_EXIdentityMappingsRejectIrrelevantValues(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "semaphore_ex_oidc_group_mapping" "test" {
  id = "oidc-invalid"
  provider_id = "acceptance-oidc"
  claim_value = "operators"
  group_external_id = "not-applicable"
  enabled = true
  target = { scope = "global", role_id = "role-static" }
}`,
				ExpectError: regexp.MustCompile("group_external_id"),
			},
			{
				Config: `
resource "semaphore_ex_ldap_group_mapping" "test" {
  id = "ldap-invalid"
  provider_id = "acceptance-directory"
  group_external_id = "entryuuid:12345678-1234-1234-1234-123456789abc"
  claim_value = "not-applicable"
  enabled = true
  target = { scope = "global", role_id = "role-static" }
}`,
				ExpectError: regexp.MustCompile("claim_value"),
			},
		},
	})
}

func TestAcc_EXTOTPPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: `resource "semaphore_ex_totp_policy" "test" { state = "disabled" }`, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("semaphore_ex_totp_policy.test", "id", "totp"),
				resource.TestCheckResourceAttr("semaphore_ex_totp_policy.test", "state", "disabled"),
			)},
			{Config: `resource "semaphore_ex_totp_policy" "test" { state = "optional" }`, Check: resource.TestCheckResourceAttr("semaphore_ex_totp_policy.test", "state", "optional")},
			{ResourceName: "semaphore_ex_totp_policy.test", ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAcc_EXLDAPConfigurationDisabled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: ldapAcceptanceConfig(t), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_ldap_configuration.test", "state", "disabled"), resource.TestCheckResourceAttr("semaphore_ex_ldap_configuration.test", "bind_password_configured", "true"))}},
	})
}

func TestAcc_EXOIDCGroupMapping(t *testing.T) {
	config := `
resource "semaphore_ex_global_role" "test" {
  name = "OIDC Acceptance Role"
  project_permissions = []
  global_permissions = ["global.audit.read"]
}
resource "semaphore_ex_oidc_group_mapping" "test" {
  id = "oidc-operators"
  provider_id = "acceptance-oidc"
  claim_value = "operators"
  enabled = true
  target = {
    scope = "global"
    role_id = semaphore_ex_global_role.test.id
  }
}`
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_oidc_group_mapping.test", "claim_value", "operators"), resource.TestCheckResourceAttrSet("semaphore_ex_oidc_group_mapping.test", "revision"))}, {Config: strings.Replace(config, "enabled = true", "enabled = false", 1), Check: resource.TestCheckResourceAttr("semaphore_ex_oidc_group_mapping.test", "enabled", "false")}, {ResourceName: "semaphore_ex_oidc_group_mapping.test", ImportState: true, ImportStateVerify: true, ImportStateId: "acceptance-oidc/mapping/oidc-operators"}},
	})
}

func TestAcc_EXLDAPGroupMapping(t *testing.T) {
	config := ldapAcceptanceConfig(t) + `
resource "semaphore_ex_global_role" "test" {
  name = "LDAP Acceptance Role"
  project_permissions = []
  global_permissions = ["global.audit.read"]
}
resource "semaphore_ex_ldap_group_mapping" "test" {
  id = "ldap-operators"
  provider_id = semaphore_ex_ldap_configuration.test.id
  group_external_id = "entryuuid:12345678-1234-1234-1234-123456789abc"
  enabled = true
  target = { scope = "global", role_id = semaphore_ex_global_role.test.id }
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.TestCheckResourceAttrSet("semaphore_ex_ldap_group_mapping.test", "revision")}, {Config: strings.Replace(config, "enabled = true", "enabled = false", 1), Check: resource.TestCheckResourceAttr("semaphore_ex_ldap_group_mapping.test", "enabled", "false")}}})
}

func ldapAcceptanceConfig(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`resource "semaphore_ex_ldap_configuration" "test" {
  id = "acceptance-directory"
  display_name = "Acceptance Directory"
  state = "disabled"
  server_url = "ldaps://directory.example.test:636"
  tls_mode = "ldaps"
  trust_mode = "custom_ca"
  ca_pem = <<-PEM
%s
PEM
  bind_dn = "cn=service,dc=example,dc=test"
  bind_password = "test-only-secret"
  search_base_dn = "ou=people,dc=example,dc=test"
  user_filter = "(uid={{username}})"
  identity_attribute = "entryUUID"
  username_attribute = "uid"
  name_attribute = "cn"
  email_attribute = "mail"
  group_search_base_dn = "ou=groups,dc=example,dc=test"
  group_user_filter = "(objectClass=person)"
  group_filter = "(objectClass=groupOfNames)"
  group_identity_attribute = "entryUUID"
  group_member_attribute = "member"
  group_max_depth = 4
}
`, ldapAcceptanceCertificatePEM(t))
}

func ldapAcceptanceCertificatePEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	certificate, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
	}, &x509.Certificate{}, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}))
}
