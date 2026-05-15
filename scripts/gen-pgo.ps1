<#
.SYNOPSIS
    Regenera o perfil PGO consumido pelo build de produção.

.DESCRIPTION
    Roda o bench BenchmarkFraudFractionRBinIVF e grava o cpuprofile em
    cmd/api/default.pgo. Esse arquivo é consumido por
    `go build -pgo=auto` no Dockerfile.hub.

    Regerar quando mudar internal/knn/*.go, internal/app/index.go ou
    internal/httpserver/server.go.

.PARAMETER BenchTime
    Quanto tempo rodar o bench (ex.: "5s", "10s"). Default: 5s.

.EXAMPLE
    .\scripts\gen-pgo.ps1
    .\scripts\gen-pgo.ps1 -BenchTime 10s
#>
[CmdletBinding()]
param(
    [string]$BenchTime = "5s"
)

$ErrorActionPreference = "Stop"
$env:GOTOOLCHAIN = "local"

Write-Host "==> rodando bench (-benchtime=$BenchTime)" -ForegroundColor Cyan

go test ./internal/knn/ -run="^$" -bench=BenchmarkFraudFractionRBinIVF `
    -benchtime=$BenchTime -outputdir=cmd/api -cpuprofile=default.pgo

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERRO: bench falhou" -ForegroundColor Red
    exit $LASTEXITCODE
}

# Workaround: no Windows o `go test` salva como "default" sem extensão.
if (Test-Path cmd/api/default) {
    Move-Item cmd/api/default cmd/api/default.pgo -Force
}

if (-not (Test-Path cmd/api/default.pgo)) {
    Write-Host "ERRO: cmd/api/default.pgo não foi gerado" -ForegroundColor Red
    exit 1
}

$size = (Get-Item cmd/api/default.pgo).Length
Write-Host ""
Write-Host "OK  cmd/api/default.pgo ($("{0:N0}" -f $size) bytes)" -ForegroundColor Green
Write-Host ""
Write-Host "Validando build com o perfil..." -ForegroundColor Cyan
go build -trimpath -pgo=auto -ldflags="-s -w" -o nul ./cmd/api

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERRO: build com PGO falhou" -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host "OK  PGO válido, pronto para commit" -ForegroundColor Green
