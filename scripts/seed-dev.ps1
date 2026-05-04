# Seeds the dev Vault with two KV mounts, userpass auth, two users
# (alice/bob) with restricted policies, and sample secrets in each
# engine. Idempotent: enables/creates only if missing.
$ErrorActionPreference = "Stop"

$env:VAULT_ADDR = "http://127.0.0.1:8200"
$env:VAULT_TOKEN = "root-test-token"

Write-Host "==> Enabling kv-v1 at kv-v1/" -ForegroundColor Yellow
& vault secrets list -format=json | Out-Null
$mounts = vault secrets list -format=json | ConvertFrom-Json
if (-not $mounts.PSObject.Properties.Name.Contains("kv-v1/")) {
    vault secrets enable -path=kv-v1 -version=1 kv | Out-Null
}

Write-Host "==> Enabling kv-v2 at kv-v2/" -ForegroundColor Yellow
if (-not $mounts.PSObject.Properties.Name.Contains("kv-v2/")) {
    vault secrets enable -path=kv-v2 -version=2 kv | Out-Null
}

Write-Host "==> Enabling userpass auth method" -ForegroundColor Yellow
$auths = vault auth list -format=json | ConvertFrom-Json
if (-not $auths.PSObject.Properties.Name.Contains("userpass/")) {
    vault auth enable userpass | Out-Null
}

Write-Host "==> Writing alice policy" -ForegroundColor Yellow
@"
# alice can fully manage kv-v1/alice/* and read kv-v2/shared/*
path "kv-v1/alice/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "kv-v1/" {
  capabilities = ["list"]
}
path "kv-v2/data/shared/*" {
  capabilities = ["read"]
}
path "kv-v2/metadata/shared/*" {
  capabilities = ["list", "read"]
}
path "kv-v2/metadata" {
  capabilities = ["list"]
}
"@ | vault policy write alice -

Write-Host "==> Writing bob policy" -ForegroundColor Yellow
@"
# bob can fully manage kv-v2/bob/* and read kv-v1/shared/*
path "kv-v2/data/bob/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "kv-v2/metadata/bob/*" {
  capabilities = ["list", "read", "delete"]
}
path "kv-v2/metadata" {
  capabilities = ["list"]
}
path "kv-v1/shared/*" {
  capabilities = ["read", "list"]
}
path "kv-v1/" {
  capabilities = ["list"]
}
"@ | vault policy write bob -

Write-Host "==> Creating users alice and bob (password: tacos)" -ForegroundColor Yellow
vault write auth/userpass/users/alice password=tacos policies=alice | Out-Null
vault write auth/userpass/users/bob   password=tacos policies=bob   | Out-Null

Write-Host "==> Seeding kv-v1 secrets" -ForegroundColor Yellow
vault kv put kv-v1/alice/db        username=alice password=alice-db-pw    host=db.alice.local | Out-Null
vault kv put kv-v1/alice/api       token=alice-api-token-abc123          endpoint=https://api.alice.local | Out-Null
vault kv put kv-v1/shared/readme   note="kv-v1 shared mount, readable by bob" | Out-Null
vault kv put kv-v1/shared/banner   message="welcome to vault-tui demo"   owner=root | Out-Null

Write-Host "==> Seeding kv-v2 secrets" -ForegroundColor Yellow
vault kv put kv-v2/bob/db          username=bob password=bob-db-pw       host=db.bob.local | Out-Null
vault kv put kv-v2/bob/api/keys    aws_access_key=AKIAFAKE aws_secret=fake-secret-key | Out-Null
# Generate a couple versions so the version browser has data later.
vault kv put kv-v2/bob/db          username=bob password=bob-db-pw-v2    host=db.bob.local | Out-Null
vault kv put kv-v2/shared/config   region=us-east-1 environment=dev | Out-Null
vault kv put kv-v2/shared/feature  flag_a=true flag_b=false | Out-Null

Write-Host ""
Write-Host "Seeded. Try:" -ForegroundColor Green
Write-Host "  VAULT_ADDR=http://127.0.0.1:8200 vault login -method=userpass username=alice password=tacos"
Write-Host "  VAULT_ADDR=http://127.0.0.1:8200 vault login -method=userpass username=bob   password=tacos"
