<#
.SYNOPSIS
    Build + push da imagem peregrinno/heimdall com Dockerfile.hub.

.DESCRIPTION
    Reproduz exatamente o que a Rinha vai puxar:
    - linux/amd64
    - GOAMD64=v3 + PGO automático (cmd/api/default.pgo)
    - references.rbin + references.ivf embutidos

    No fim imprime o digest @sha256:... para fixar na branch submission.

.PARAMETER Image
    Nome da imagem com tag. Default: peregrinno/heimdall:latest

.EXAMPLE
    .\scripts\build-push.ps1
    .\scripts\build-push.ps1 -Image peregrinno/heimdall:v2
#>
[CmdletBinding()]
param(
    [string]$Image = "peregrinno/heimdall:latest"
)

$ErrorActionPreference = "Stop"

# Pré-cheques
foreach ($f in @("data/references.rbin", "data/references.ivf", "cmd/api/default.pgo")) {
    if (-not (Test-Path $f)) {
        Write-Host "ERRO: arquivo obrigatório ausente: $f" -ForegroundColor Red
        Write-Host "Veja docs/operacao-windows.md para regerar." -ForegroundColor Yellow
        exit 1
    }
}

$gitSha = (git rev-parse --short HEAD).Trim()
Write-Host "==> build $Image (commit $gitSha)" -ForegroundColor Cyan

docker build --no-cache --platform linux/amd64 -f Dockerfile.hub `
    --build-arg "GIT_SHA=$gitSha" `
    -t $Image .

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERRO: docker build falhou" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "==> push $Image" -ForegroundColor Cyan
docker push $Image

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERRO: docker push falhou (faz 'docker login -u peregrinno' antes)" -ForegroundColor Red
    exit $LASTEXITCODE
}

$digest = (docker image inspect $Image --format "{{(index .RepoDigests 0)}}")
$digest = ($digest -split "@")[1]

Write-Host ""
Write-Host "OK  imagem publicada" -ForegroundColor Green
Write-Host "    Digest: $digest" -ForegroundColor Green
Write-Host ""
Write-Host "Próximo passo: atualizar docker-compose.yml na branch submission" -ForegroundColor Yellow
Write-Host "    image: peregrinno/heimdall@$digest"
