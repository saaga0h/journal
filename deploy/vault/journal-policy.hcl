# Vault policy for Journal Nomad jobs.
# Apply with: vault policy write journal deploy/vault/journal-policy.hcl
#
# Note: KV v2 stores data under secret/data/<path> even though
# `vault kv put` uses secret/<path> on the CLI.

path "secret/data/nomad/journal" {
  capabilities = ["read"]
}
