param(
    [switch]$Reseed,
    [switch]$Stop
)

$ErrorActionPreference = "Stop"

$VaultAddr = "http://127.0.0.1:8210"
$VaultToken = "root-test-token"
$VaultListen = "127.0.0.1:8210"
$StateDir = Join-Path ([System.IO.Path]::GetTempPath()) "vault-tui-dev"
$StateFile = Join-Path $StateDir "state.json"
$StdOutLog = Join-Path $StateDir "vault.stdout.log"
$StdErrLog = Join-Path $StateDir "vault.stderr.log"

function Ensure-Dir([string]$Path) {
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path | Out-Null
    }
}

function Get-State {
    if (-not (Test-Path $StateFile)) {
        return $null
    }
    return Get-Content $StateFile | ConvertFrom-Json
}

function Save-State([int]$ProcessId) {
    Ensure-Dir $StateDir
    [pscustomobject]@{
        pid   = $ProcessId
        addr  = $VaultAddr
        token = $VaultToken
    } | ConvertTo-Json | Set-Content $StateFile
}

function Remove-State {
    if (Test-Path $StateFile) {
        Remove-Item $StateFile -Force
    }
}

function Stop-SeededVault {
    $state = Get-State
    if ($null -eq $state) {
        Write-Host "No seeded Vault state found." -ForegroundColor Yellow
        return
    }

    $proc = Get-Process -Id $state.pid -ErrorAction SilentlyContinue
    if ($null -ne $proc) {
        Write-Host "==> Stopping Vault dev server (PID $($state.pid))" -ForegroundColor Yellow
        Stop-Process -Id $state.pid -Force
    }
    Remove-State
}

function Test-VaultReachable {
    try {
        $null = Invoke-RestMethod -Uri "$VaultAddr/v1/sys/health?standbyok=true&sealedcode=299&uninitcode=299" -Method Get -TimeoutSec 2
        return $true
    }
    catch {
        return $false
    }
}

function Wait-ForVault {
    for ($i = 0; $i -lt 40; $i++) {
        if (Test-VaultReachable) {
            return
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Vault dev server did not become ready at $VaultAddr"
}

function Start-SeededVault {
    if (Test-VaultReachable) {
        Write-Host "==> Reusing Vault dev server already reachable at $VaultAddr" -ForegroundColor Yellow
        return
    }

    $existing = Get-State
    if ($null -ne $existing) {
        $proc = Get-Process -Id $existing.pid -ErrorAction SilentlyContinue
        if ($null -ne $proc) {
            Write-Host "==> Reusing existing Vault dev server (PID $($existing.pid))" -ForegroundColor Yellow
            return
        }
        Remove-State
    }

    Ensure-Dir $StateDir
    if (Test-Path $StdOutLog) { Remove-Item $StdOutLog -Force }
    if (Test-Path $StdErrLog) { Remove-Item $StdErrLog -Force }

    Write-Host "==> Starting temporary Vault dev server on $VaultListen" -ForegroundColor Yellow
    $proc = Start-Process vault `
        -ArgumentList @(
            "server",
            "-dev",
            "-dev-root-token-id=$VaultToken",
            "-dev-listen-address=$VaultListen"
        ) `
        -RedirectStandardOutput $StdOutLog `
        -RedirectStandardError $StdErrLog `
        -PassThru

    Save-State -ProcessId $proc.Id
    Wait-ForVault
}

function Seed-Vault {
    $env:VAULT_ADDR = $VaultAddr
    $env:VAULT_TOKEN = $VaultToken
    $env:VAULT_TUI_TEST_ADDR = $VaultAddr
    $env:VAULT_TUI_TEST_TOKEN = $VaultToken

    Write-Host "==> Enabling kv-v1 at kv-v1/" -ForegroundColor Yellow
    $mounts = vault secrets list -format=json | ConvertFrom-Json
    if (-not $mounts.PSObject.Properties.Name.Contains("kv-v1/")) {
        vault secrets enable -path=kv-v1 -version=1 kv | Out-Null
        $mounts = vault secrets list -format=json | ConvertFrom-Json
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
path "kv-v2/delete/bob/*" {
  capabilities = ["update"]
}
path "kv-v2/undelete/bob/*" {
  capabilities = ["update"]
}
path "kv-v2/destroy/bob/*" {
  capabilities = ["update"]
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
path "sys/capabilities-self" {
  capabilities = ["update"]
}
"@ | vault policy write bob -

    Write-Host "==> Creating users alice and bob" -ForegroundColor Yellow
    vault write auth/userpass/users/alice password=hunter2 policies=alice | Out-Null
    vault write auth/userpass/users/bob   password=tacos   policies=bob   | Out-Null

    if ($Reseed) {
        Write-Host "==> Clearing prior integration test data" -ForegroundColor Yellow
        vault kv metadata delete kv-v2/vault-tui-itest/delete-all 2>$null | Out-Null
        vault kv metadata delete kv-v2/vault-tui-itest/rollback 2>$null | Out-Null
        vault kv metadata delete kv-v2/vault-tui-itest/version-lifecycle 2>$null | Out-Null
        vault kv metadata delete kv-v2/vault-tui-itest/v2secret 2>$null | Out-Null
        vault kv delete kv-v1/vault-tui-itest/v1secret 2>$null | Out-Null
    }

    Write-Host "==> Seeding kv-v1 secrets" -ForegroundColor Yellow
    vault kv put kv-v1/alice/db      username=alice password=alice-db-pw host=db.alice.local | Out-Null
    vault kv put kv-v1/alice/api     token=alice-api-token-abc123 endpoint=https://api.alice.local | Out-Null
    vault kv put kv-v1/shared/readme note="kv-v1 shared mount, readable by bob" | Out-Null
    vault kv put kv-v1/shared/banner message="welcome to vault-tui demo" owner=root | Out-Null

    Write-Host "==> Seeding kv-v2 secrets" -ForegroundColor Yellow
    vault kv put kv-v2/bob/db       username=bob password=bob-db-pw host=db.bob.local | Out-Null
    vault kv put kv-v2/bob/api/keys aws_access_key=AKIAFAKE aws_secret=fake-secret-key | Out-Null
    vault kv put kv-v2/bob/db       username=bob password=bob-db-pw-v2 host=db.bob.local | Out-Null
    vault kv put kv-v2/shared/config  region=us-east-1 environment=dev | Out-Null
    vault kv put kv-v2/shared/feature flag_a=true flag_b=false | Out-Null

    Write-Host "==> Seeding dedicated integration test fixtures" -ForegroundColor Yellow
    vault kv put kv-v2/vault-tui-itest/version-lifecycle k=v1 | Out-Null
    vault kv put kv-v2/vault-tui-itest/version-lifecycle k=v2 | Out-Null
    vault kv put kv-v2/vault-tui-itest/version-lifecycle k=v3 | Out-Null

    vault kv put kv-v2/vault-tui-itest/rollback k=v1 | Out-Null
    vault kv put kv-v2/vault-tui-itest/rollback k=v2 | Out-Null

    vault kv put kv-v2/vault-tui-itest/delete-all k=v1 | Out-Null
    vault kv put kv-v2/vault-tui-itest/delete-all k=v2 | Out-Null

    Write-Host ""
    Write-Host "Seeded temporary Vault dev server." -ForegroundColor Green
    Write-Host "  VAULT_ADDR=$VaultAddr"
    Write-Host "  VAULT_TOKEN=$VaultToken"
    Write-Host "  VAULT_TUI_TEST_ADDR=$VaultAddr"
    Write-Host "  VAULT_TUI_TEST_TOKEN=$VaultToken"
    Write-Host ""
    Write-Host "Userpass test logins:" -ForegroundColor Green
    Write-Host "  alice / hunter2"
    Write-Host "  bob   / tacos"
    Write-Host ""
    Write-Host "To stop the temporary server:" -ForegroundColor Green
    Write-Host "  .\seed-dev.ps1 -Stop"
}

if ($Stop) {
    Stop-SeededVault
    return
}

Start-SeededVault
Seed-Vault
