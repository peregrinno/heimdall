package vector

import (
	"math"
	"testing"
	"time"

	"heimdall/internal/config"
	"heimdall/internal/domain"
)

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 5e-4
}

func TestBuild_LegitOverviewExample(t *testing.T) {
	n := config.Normalization{
		MaxAmount:            10000,
		MaxInstallments:      12,
		AmountVsAvgRatio:     10,
		MaxMinutes:           1440,
		MaxKm:                1000,
		MaxTxCount24h:        20,
		MaxMerchantAvgAmount: 10000,
	}
	mcc := config.MCCRisk{"5411": 0.15}

	reqAt, _ := time.Parse(time.RFC3339, "2026-03-11T18:45:53Z")
	req := domain.FraudScoreRequest{
		ID: "tx-1329056812",
		Transaction: domain.Transaction{
			Amount:       41.12,
			Installments: 2,
			RequestedAt:  reqAt,
		},
		Customer: domain.Customer{
			AvgAmount:      82.24,
			TxCount24h:     3,
			KnownMerchants: []string{"MERC-003", "MERC-016"},
		},
		Merchant: domain.Merchant{
			ID:        "MERC-016",
			MCC:       "5411",
			AvgAmount: 60.25,
		},
		Terminal: domain.Terminal{
			IsOnline:    false,
			CardPresent: true,
			KmFromHome:  29.23,
		},
		LastTransaction: nil,
	}

	got := Build(req, n, mcc)
	want := [14]float64{0.0041, 0.1667, 0.05, 0.7826, 0.3333, -1, -1, 0.0292, 0.15, 0, 1, 0, 0.15, 0.006}

	for i := range want {
		if want[i] == -1 {
			if got[i] != -1 {
				t.Fatalf("dim %d: got %v want -1", i, got[i])
			}
			continue
		}
		if !approxEq(got[i], want[i]) {
			t.Fatalf("dim %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestBuild_FraudOverviewExample(t *testing.T) {
	n := config.Normalization{
		MaxAmount:            10000,
		MaxInstallments:      12,
		AmountVsAvgRatio:     10,
		MaxMinutes:           1440,
		MaxKm:                1000,
		MaxTxCount24h:        20,
		MaxMerchantAvgAmount: 10000,
	}
	mcc := config.MCCRisk{"7802": 0.75}

	reqAt, _ := time.Parse(time.RFC3339, "2026-03-14T05:15:12Z")
	req := domain.FraudScoreRequest{
		ID: "tx-3330991687",
		Transaction: domain.Transaction{
			Amount:       9505.97,
			Installments: 10,
			RequestedAt:  reqAt,
		},
		Customer: domain.Customer{
			AvgAmount:      81.28,
			TxCount24h:     20,
			KnownMerchants: []string{"MERC-008", "MERC-007", "MERC-005"},
		},
		Merchant: domain.Merchant{
			ID:        "MERC-068",
			MCC:       "7802",
			AvgAmount: 54.86,
		},
		Terminal: domain.Terminal{
			IsOnline:    false,
			CardPresent: true,
			KmFromHome:  952.27,
		},
		LastTransaction: nil,
	}

	got := Build(req, n, mcc)
	want := [14]float64{0.9506, 0.8333, 1.0, 0.2174, 0.8333, -1, -1, 0.9523, 1.0, 0, 1, 1, 0.75, 0.0055}

	for i := range want {
		if want[i] == -1 {
			if got[i] != -1 {
				t.Fatalf("dim %d: got %v want -1", i, got[i])
			}
			continue
		}
		if !approxEq(got[i], want[i]) {
			t.Fatalf("dim %d: got %v want %v", i, got[i], want[i])
		}
	}
}
