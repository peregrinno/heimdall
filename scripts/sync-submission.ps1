<#
.SYNOPSIS
    Sincroniza a branch submission com o digest da imagem do Hub
    e copia warmup.sh + haproxy.cfg da main.

.DESCRIPTION
    Pré-requisitos:
    - git worktree em ..\heimdall-submission apontando para a branch submission
        git worktree add ..\heimdall-submission submission
    - imagem publicada no Hub com o digest desejado
        .\scripts\build-push.ps1

    O script:
    1. Lê o digest atual de peregrinno/heimdall:latest.
    2. No worktree submission, troca a linha `image:` para usar @sha256:...
    3. Copia scripts/warmup.sh e deploy/haproxy.cfg da main.
    4. Faz commit + push origin submission.

.PARAMETER Image
    Imagem com tag. Default: peregrinno/heimdall:latest

.PARAMETER Worktree
    Caminho do worktree da submission. Default: ..\heimdall-submission

.EXAMPLE
    .\scripts\sync-submission.ps1
#>
[CmdletBinding()]
param(
    [string]$Image = "peregrinno/heimdall:latest",
    [string]$Worktree = "..\heimdall-submission"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $Worktree)) {
    Write-Host "ERRO: worktree não existe em $Worktree" -ForegroundColor Red
    Write-Host "Crie com:  git worktree add $Worktree submission" -ForegroundColor Yellow
    exit 1
}

# Pega digest do Hub via inspect local (precisa de docker pull/push prévio)
$digest = (docker image inspect $Image --format "{{(index .RepoDigests 0)}}" 2>$null)
if (-not $digest) {
    Write-Host "ERRO: não consegui ler o digest de $Image (faça push primeiro)" -ForegroundColor Red
    exit 1
}
$digest = ($digest -split "@")[1]
$short = $digest.Substring(7, 12)

Write-Host "==> digest atual: $digest" -ForegroundColor Cyan

$compose = Join-Path $Worktree "docker-compose.yml"
if (-not (Test-Path $compose)) {
    Write-Host "ERRO: $compose não existe" -ForegroundColor Red
    exit 1
}

# Substitui linhas `image: peregrinno/heimdall...`
$lines = Get-Content $compose
$newLines = $lines | ForEach-Object {
    if ($_ -match '^\s*image:\s*peregrinno/heimdall') {
        $_ -replace 'peregrinno/heimdall(@sha256:[a-f0-9]+|:[\w\.-]+)', "peregrinno/heimdall@$digest"
    } else {
        $_
    }
}
Set-Content $compose -Value $newLines -Encoding UTF8

# Copia ficheiros suportes
Copy-Item .\scripts\warmup.sh   (Join-Path $Worktree "scripts\warmup.sh")  -Force
Copy-Item .\deploy\haproxy.cfg  (Join-Path $Worktree "deploy\haproxy.cfg") -Force

Write-Host "==> commit + push na submission" -ForegroundColor Cyan
Push-Location $Worktree
try {
    git add .
    $diff = git diff --staged --name-only
    if (-not $diff) {
        Write-Host "Nada a comitar." -ForegroundColor Yellow
        return
    }
    git commit -m "submission: pin @ $short + sync haproxy/warmup"
    git push origin submission
    Write-Host ""
    Write-Host "OK  submission atualizada com digest $short" -ForegroundColor Green
}
finally {
    Pop-Location
}
