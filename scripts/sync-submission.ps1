<#
.SYNOPSIS
    Sincroniza a branch submission com o digest da imagem do Hub.

.DESCRIPTION
    Pré-requisitos:
    - git worktree em ..\heimdall-submission apontando para a branch submission:
        git worktree add ..\heimdall-submission submission
    - imagem publicada no Hub com o digest desejado:
        .\scripts\build-push.ps1

    O script:
    1. Lê o digest atual de peregrinno/heimdall:latest (via docker image inspect).
    2. Copia deploy/docker-compose.submission.yml -> ..\heimdall-submission\docker-compose.yml
       substituindo PLACEHOLDER_DIGEST pelo digest real.
    3. Copia scripts/warmup.sh, deploy/haproxy.cfg, .gitattributes.
    4. Faz commit + push origin submission.

    Idempotência:
    - O template canônico (deploy/docker-compose.submission.yml) é a fonte
      de verdade para a submission. Nunca usar Copy-Item do docker-compose.yml
      da main (que tem `build: .` e quebra o smoke test da Rinha).

.PARAMETER Image
    Imagem com tag. Default: peregrinno/heimdall:latest

.PARAMETER Worktree
    Caminho do worktree da submission. Default: ..\heimdall-submission

.PARAMETER Digest
    Digest sha256 a usar. Se omitido, lê de `docker image inspect $Image`.
    Útil para forçar um digest específico sem precisar pull/push antes.

.EXAMPLE
    .\scripts\sync-submission.ps1

.EXAMPLE
    .\scripts\sync-submission.ps1 -Digest sha256:ea1bc3b67d35f864aa018d457c9a2fc5594c51a24e0d49695721aee99484b23e
#>
[CmdletBinding()]
param(
    [string]$Image    = "peregrinno/heimdall:latest",
    [string]$Worktree = "..\heimdall-submission",
    [string]$Digest   = ""
)

$ErrorActionPreference = "Stop"

function Invoke-Native {
    param([scriptblock]$Block)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { & $Block } finally { $ErrorActionPreference = $prev }
    if ($LASTEXITCODE -ne 0) {
        throw "comando nativo falhou (exit=$LASTEXITCODE)"
    }
}

if (-not (Test-Path $Worktree)) {
    Write-Host "ERRO: worktree nao existe em $Worktree" -ForegroundColor Red
    Write-Host "Crie com:  git worktree add $Worktree submission" -ForegroundColor Yellow
    exit 1
}

$template = "deploy\docker-compose.submission.yml"
if (-not (Test-Path $template)) {
    Write-Host "ERRO: template $template nao existe." -ForegroundColor Red
    exit 1
}

if (-not $Digest) {
    Write-Host "==> lendo digest de $Image" -ForegroundColor Cyan
    $repoDigest = (docker image inspect $Image --format "{{(index .RepoDigests 0)}}" 2>$null)
    if (-not $repoDigest) {
        Write-Host "ERRO: docker image inspect $Image nao retornou RepoDigest." -ForegroundColor Red
        Write-Host "Faca push primeiro:  .\scripts\build-push.ps1" -ForegroundColor Yellow
        exit 1
    }
    $Digest = ($repoDigest -split "@")[1]
}

if ($Digest -notmatch '^sha256:[a-f0-9]{64}$') {
    Write-Host "ERRO: digest invalido: $Digest" -ForegroundColor Red
    Write-Host "Esperado formato: sha256:<64-hex>" -ForegroundColor Yellow
    exit 1
}

$short = $Digest.Substring(7, 12)
Write-Host "==> digest: $Digest" -ForegroundColor Cyan
Write-Host "==> short:  $short" -ForegroundColor Cyan

# 1. compose: le template em UTF-8, substitui placeholder, grava na submission
$composeOut    = Join-Path $Worktree "docker-compose.yml"
$composeOutAbs = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $composeOut))
$templateAbs   = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $template))
# ReadAllText sem BOM force UTF-8 (Get-Content -Raw no Windows lê como cp1252 e gera mojibake).
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$content   = [System.IO.File]::ReadAllText($templateAbs, $utf8NoBom)
$content   = $content -replace 'sha256:PLACEHOLDER_DIGEST', $Digest
# WriteAllText sem BOM preserva LF + UTF-8 (previsivel cross-platform).
[System.IO.File]::WriteAllText($composeOutAbs, $content, $utf8NoBom)

# Sanidade: confere que nao restou PLACEHOLDER e que tem o digest 2x (api-1, api-2).
$check = Get-Content $composeOut -Raw
if ($check -match 'PLACEHOLDER_DIGEST') {
    Write-Host "ERRO: substituicao falhou, ainda ha PLACEHOLDER_DIGEST em $composeOut" -ForegroundColor Red
    exit 1
}
$digestCount = ([regex]::Matches($check, [regex]::Escape($Digest))).Count
if ($digestCount -lt 2) {
    Write-Host "ERRO: esperava o digest 2x (api-1 + api-2), encontrei $digestCount" -ForegroundColor Red
    exit 1
}
Write-Host "==> compose escrito ($digestCount referencias ao digest)" -ForegroundColor Green

# 2. arquivos de suporte
$pairs = @(
    @{ Src = "scripts\warmup.sh";     Dst = "scripts\warmup.sh" },
    @{ Src = "deploy\haproxy.cfg";    Dst = "deploy\haproxy.cfg" },
    @{ Src = ".gitattributes";        Dst = ".gitattributes" }
)
foreach ($p in $pairs) {
    $dst = Join-Path $Worktree $p.Dst
    $dstDir = Split-Path $dst
    if (-not (Test-Path $dstDir)) { New-Item -ItemType Directory -Force -Path $dstDir | Out-Null }
    Copy-Item $p.Src $dst -Force
    Write-Host "==> copiado $($p.Src) -> $dst" -ForegroundColor DarkGray
}

# 3. commit + push
Write-Host "==> commit + push na submission" -ForegroundColor Cyan
Push-Location $Worktree
try {
    Invoke-Native { git add . 2>&1 | Out-Null }
    $diff = & git diff --staged --name-only 2>$null
    if (-not $diff) {
        Write-Host "Nada a comitar (worktree ja estava sincronizado)." -ForegroundColor Yellow
        return
    }
    Invoke-Native { git commit -m "submission: pin @ $short + sync support files" 2>&1 | Out-Null }
    Invoke-Native { git push origin submission 2>&1 | Out-Null }
    Write-Host ""
    Write-Host "OK  submission atualizada com digest $short" -ForegroundColor Green
}
finally {
    Pop-Location
}
