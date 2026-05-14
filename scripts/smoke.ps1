$ErrorActionPreference = "Stop"
$base = "http://localhost:9999"

Write-Host "GET $base/ready" -ForegroundColor Cyan
$r = Invoke-WebRequest -Uri "$base/ready" -UseBasicParsing
if ($r.StatusCode -ne 200) {
  Write-Error "ready falhou: $($r.StatusCode)"
}

$body = @{
  id = "tx-smoke-1"
  transaction = @{
    amount = 384.88
    installments = 3
    requested_at = "2026-03-11T20:23:35Z"
  }
  customer = @{
    avg_amount = 769.76
    tx_count_24h = 3
    known_merchants = @("MERC-009", "MERC-001")
  }
  merchant = @{
    id = "MERC-001"
    mcc = "5912"
    avg_amount = 298.95
  }
  terminal = @{
    is_online = $false
    card_present = $true
    km_from_home = 13.7
  }
  last_transaction = @{
    timestamp = "2026-03-11T14:58:35Z"
    km_from_current = 18.8
  }
} | ConvertTo-Json -Depth 6

Write-Host "POST $base/fraud-score" -ForegroundColor Cyan
$p = Invoke-WebRequest -Uri "$base/fraud-score" -Method POST -Body $body `
  -ContentType "application/json; charset=utf-8" -UseBasicParsing
Write-Host $p.Content
