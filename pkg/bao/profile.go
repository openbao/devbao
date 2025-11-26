package bao

import (
	"context"
	"fmt"
	"strings"

	"github.com/openbao/openbao/api/v2"
)

const (
	PKIProfile      string = "pki"
	TransitProfile  string = "transit"
	UserpassProfile string = "userpass"
	SecretProfile   string = "secret"
)

func ListProfiles() []string {
	return []string{
		PKIProfile,
		TransitProfile,
		UserpassProfile,
		SecretProfile,
	}
}

func ProfileDescription(name string) string {
	switch name {
	case PKIProfile:
		return "enable a two-tier root & intermediate CA hierarchy"
	case TransitProfile:
		return "enable transit for auto-unseal of another cluster"
	case UserpassProfile:
		return "enable userpass authentication and sample policy"
	case SecretProfile:
		return "enable a KVv2 static secret engine"
	}

	return ""
}

func ProfileSetup(client *api.Client, profile string) ([]string, error) {
	switch strings.ToLower(profile) {
	case PKIProfile:
		return ProfilePKIMountSetup(client)
	case TransitProfile:
		return ProfileTransitSealMountSetup(client)
	case UserpassProfile:
		return ProfileUserpassMountSetup(client)
	case SecretProfile:
		return ProfileSecretMountSetup(client)
	default:
		return nil, fmt.Errorf("unknown profile to apply: %v", profile)
	}
}

func ProfileRemove(client *api.Client, profile string) ([]string, error) {
	switch strings.ToLower(profile) {
	case PKIProfile:
		return ProfilePKIMountRemove(client)
	case TransitProfile:
		return ProfileTransitSealMountRemove(client)
	case UserpassProfile:
		return ProfileUserpassMountRemove(client)
	case SecretProfile:
		return ProfileSecretMountRemove(client)
	default:
		return nil, fmt.Errorf("unknown profile to apply: %v", profile)
	}
}

func ProfileTransitSealMountSetup(client *api.Client) ([]string, error) {
	if err := client.Sys().Mount("transit", &api.MountInput{
		Type: "transit",
	}); err != nil {
		return nil, fmt.Errorf("failed to mount transit instance: %w", err)
	}

	resp, err := client.Logical().Write("transit/keys/auto-unseal", map[string]interface{}{
		"type": "aes256-gcm96",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create transit unseal key: %w", err)
	}

	return resp.Warnings, nil
}

func ProfileTransitSealMountRemove(client *api.Client) ([]string, error) {
	if err := client.Sys().Unmount("transit"); err != nil {
		return nil, fmt.Errorf("failed to remove transit mount: %w", err)
	}

	return nil, nil
}

func ProfilePKIMountSetup(client *api.Client) ([]string, error) {
	var warnings []string

	// Orders of operation
	//
	// 1. Create & tune root (pki-root),
	// 2. Create & sign intermediate (pki-int), and
	// 3. Create a role "testing" in the intermediate.

	// 1. Mount the root.
	if err := client.Sys().Mount("pki-root", &api.MountInput{
		Type: "pki",
		Config: api.MountConfigInput{
			MaxLeaseTTL: "87600h", /* 10y */
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to mount pki root instance: %w", err)
	}

	// Build root CA, saving it for later.
	rootResp, err := client.Logical().Write("pki-root/root/generate/internal", map[string]interface{}{
		"common_name": "Example Root X1",
		"issuer_name": "root-x1",
		"key_name":    "key-root-x1",

		// P-256
		"key_type": "ec",
		"key_bits": "256",

		"ttl": "87600h", /* 10y */
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to generate root CA: %w", err)
	}
	if len(rootResp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/root/generate/internal:\n\t", rootResp.Warnings...)
	}

	// --> Patch it to allow infinite leaf not after behavior since it is a
	// root CA.
	resp, err := client.Logical().JSONMergePatch(context.Background(), "pki-root/issuer/root-x1", map[string]interface{}{
		"leaf_not_after_behavior": "permit",
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set root CA cluster urls: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/issuer/root-x1:\n\t", resp.Warnings...)
	}

	// Enable root mount AIA information & revocation. ACME is not enabled
	// on the root to encourage use of the intermediate.
	resp, err = client.Logical().Write("pki-root/config/cluster", map[string]interface{}{
		"path":     fmt.Sprintf("%v/v1/pki-root", client.Address()),
		"aia_path": fmt.Sprintf("%v/v1/pki-root", client.Address()),
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set root CA cluster urls: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/config/cluster:\n\t", resp.Warnings...)
	}

	resp, err = client.Logical().Write("pki-root/config/urls", map[string]interface{}{
		"issuing_certificates":    "{{cluster_aia_path}}/issuer/{{issuer_id}}/der",
		"crl_distribution_points": "{{cluster_aia_path}}/issuer/{{issuer_id}}/crl/der",
		"ocsp_servers":            "{{cluster_aia_path}}/ocsp",
		"enable_templating":       true,
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set root CA urls: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/config/urls:\n\t", resp.Warnings...)
	}

	resp, err = client.Logical().Write("pki-root/config/crl", map[string]interface{}{
		"auto_rebuild": true,
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set root CA CRL config: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/config/crl:\n\t", resp.Warnings...)
	}

	// 2. Mount the intermediate CA.
	if err := client.Sys().Mount("pki-int", &api.MountInput{
		Type: "pki",
		Config: api.MountConfigInput{
			MaxLeaseTTL: "2160h", /* 180d */
			PassthroughRequestHeaders: []string{
				"If-Modified-Since",
			},
			AllowedResponseHeaders: []string{
				"Replay-Nonce",
				"Link",
				"Location",
				"Last-Modified",
			},
		},
	}); err != nil {
		return warnings, fmt.Errorf("failed to mount pki intermediate instance: %w", err)
	}

	// Enable intermediate mount AIA information & ACME & CRLs.
	resp, err = client.Logical().Write("pki-int/config/cluster", map[string]interface{}{
		"path":     fmt.Sprintf("%v/v1/pki-int", client.Address()),
		"aia_path": fmt.Sprintf("%v/v1/pki-int", client.Address()),
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set int CA cluster urls: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/config/cluster:\n\t", resp.Warnings...)
	}

	resp, err = client.Logical().Write("pki-int/config/urls", map[string]interface{}{
		"issuing_certificates":    "{{cluster_aia_path}}/issuer/{{issuer_id}}/der",
		"crl_distribution_points": "{{cluster_aia_path}}/issuer/{{issuer_id}}/crl/der",
		"ocsp_servers":            "{{cluster_aia_path}}/ocsp",
		"enable_templating":       true,
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set int CA urls: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/config/urls:\n\t", resp.Warnings...)
	}

	resp, err = client.Logical().Write("pki-int/config/acme", map[string]interface{}{
		"enabled": true,
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set int CA ACME config: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/config/acme:\n\t", resp.Warnings...)
	}

	resp, err = client.Logical().Write("pki-int/config/crl", map[string]interface{}{
		"auto_rebuild": true,
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to set int CA CRL config: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/config/crl:\n\t", resp.Warnings...)
	}

	// Create the intermediate CA
	//
	// -> Create the CSR
	intCSRResp, err := client.Logical().Write("pki-int/intermediate/generate/internal", map[string]interface{}{
		"common_name": "Example Int R1",
		"key_name":    "key-int-r1",

		// P-256
		"key_type": "ec",
		"key_bits": "256",
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to create intermediate CA CSR: %w", err)
	}
	if len(intCSRResp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/intermediate/generate/internal:\n\t", intCSRResp.Warnings...)
	}

	// -> Sign the CSR with the root mount
	intCAResp, err := client.Logical().Write("pki-root/root/sign-intermediate", map[string]interface{}{
		"csr": intCSRResp.Data["csr"],

		"ttl": "4380h", /* 6mo */
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to sign the intermediate CA CSR with the root CA: %w", err)
	}
	if len(intCAResp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-root/root/sign-intermediate:\n\t", intCAResp.Warnings...)
	}

	// -> Import the intermediate into its mount
	resp, err = client.Logical().Write("pki-int/issuers/import/cert", map[string]interface{}{
		"pem_bundle": intCAResp.Data["certificate"],
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to import intermediate CA: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/issuers/import/cert (intermediate import):\n\t", resp.Warnings...)
	}

	// -> Set the intermediate's name, leaf-not-after behavior
	resp, err = client.Logical().JSONMergePatch(context.Background(), "pki-int/issuer/default", map[string]interface{}{
		"issuer_name":             "int-r1",
		"leaf_not_after_behavior": "truncate",
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to rename intermediate CA: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/issuer/default:\n\t", resp.Warnings...)
	}

	// -> Import the root, find its identifier
	rootImportResp, err := client.Logical().Write("pki-int/issuers/import/cert", map[string]interface{}{
		"pem_bundle": rootResp.Data["certificate"],
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to import root CA cert into intermediate mount: %w", err)
	}
	if len(rootImportResp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/issuers/import/cert (root import):\n\t", rootImportResp.Warnings...)
	}

	importedIssuersRaw := rootImportResp.Data["imported_issuers"].([]interface{})
	if len(importedIssuersRaw) == 0 {
		return warnings, fmt.Errorf("root issuer was not imported: response from pki-int/issuers/import/cert:\n\t%v", rootImportResp.Data)
	}

	rootIssuerId := importedIssuersRaw[0].(string)

	// -> Configure the root's name in this mount.
	resp, err = client.Logical().JSONMergePatch(context.Background(), "pki-int/issuer/"+rootIssuerId, map[string]interface{}{
		"issuer_name": "root-r1",
	})
	if err != nil {
		return warnings, fmt.Errorf("failed to rename root CA: %w", err)
	}
	if len(resp.Warnings) > 0 {
		warnings = PrefixedAppend(warnings, "from pki-int/issuer/"+rootIssuerId+" (rename root):\n\t", resp.Warnings...)
	}

	// 3. Finally, create a fairly permissive role in the intermediate.
	resp, err = client.Logical().Write("pki-int/roles/testing", map[string]interface{}{
		"allow_any_name":    true,
		"enforce_hostnames": false,
		"key_type":          "any",
		"ttl":               "2160h", /* 90 days */
	})

	return warnings, nil
}

func ProfilePKIMountRemove(client *api.Client) ([]string, error) {
	if err := client.Sys().Unmount("pki-int"); err != nil {
		return nil, fmt.Errorf("failed to remove intermediate CA mount: %w", err)
	}

	if err := client.Sys().Unmount("pki-root"); err != nil {
		return nil, fmt.Errorf("failed to remove root CA mount: %w", err)
	}

	return nil, nil
}

var adminPolicy = `
path "*" {
	capabilities  = ["create", "update", "delete", "read", "patch", "list", "sudo"]
}
`

var examplePolicy = `
path "pki-int/issue/testing" {
	capabilities = ["update"]
}

path "pki-int/sign/testing" {
	capabilities = ["update"]
}

path "transit/keys/testing-key" {
	capabilities  = ["create", "update", "delete", "read"]
}

path "transit/sign/testing-key" {
	capabilities = ["create", "update"]
}

path "transit/verify/testing-key" {
	capabilities = ["create", "update"]
}

path "transit/encrypt/testing-key" {
	capabilities = ["create", "update"]
}

path "transit/decrypt/testing-key" {
	capabilities = ["create", "update"]
}

path "transit/hash" {
	capabilities = ["create", "update"]
}

path "transit/hash/*" {
	capabilities = ["create", "update"]
}

path "transit/random" {
	capabilities = ["create", "update"]
}

path "transit/random/*" {
	capabilities = ["create", "update"]
}

path "secret/+/scratch/*" {
	capabilities = ["create", "read", "update", "patch", "list", "scan"]
}
`

func ProfileUserpassMountSetup(client *api.Client) ([]string, error) {
	if err := client.Sys().EnableAuthWithOptions("userpass", &api.EnableAuthOptions{
		Type: "userpass",
	}); err != nil {
		return nil, fmt.Errorf("failed to mount userpass instance: %w", err)
	}

	if err := client.Sys().PutPolicy("example", examplePolicy); err != nil {
		return nil, fmt.Errorf("failed to create `example` ACL policy: %w", err)
	}

	if err := client.Sys().PutPolicy("admin", adminPolicy); err != nil {
		return nil, fmt.Errorf("failed to create `admin` ACL policy: %w", err)
	}

	// Userpass doesn't return a result here on account creation.
	_, err := client.Logical().Write("auth/userpass/users/testing", map[string]interface{}{
		"password":       "testing",
		"token_policies": "example",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create userpass `testing` user: %w", err)
	}

	_, err = client.Logical().Write("auth/userpass/users/admin", map[string]interface{}{
		"password":       "admin",
		"token_policies": "admin",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create userpass `admin` user: %w", err)
	}

	return []string{
		"authentication available with (u: testing/p: testing) and (u: admin/p: admin)",
	}, nil
}

func ProfileUserpassMountRemove(client *api.Client) ([]string, error) {
	if err := client.Sys().DisableAuth("userpass"); err != nil {
		return nil, fmt.Errorf("failed to remove userpass mount: %w", err)
	}

	return nil, nil
}

func ProfileSecretMountSetup(client *api.Client) ([]string, error) {
	if err := client.Sys().Mount("secret", &api.MountInput{
		Type: "kv-v2",
	}); err != nil {
		return nil, fmt.Errorf("failed to mount kv2 instance: %w", err)
	}

	return nil, nil
}

func ProfileSecretMountRemove(client *api.Client) ([]string, error) {
	if err := client.Sys().Unmount("secret"); err != nil {
		return nil, fmt.Errorf("failed to remove secret mount: %w", err)
	}

	return nil, nil
}
