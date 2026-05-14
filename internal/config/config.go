package config

import (
	"encoding/json"
	"os"
)

type Normalization struct {
	MaxAmount            float64 `json:"max_amount"`
	MaxInstallments      float64 `json:"max_installments"`
	AmountVsAvgRatio     float64 `json:"amount_vs_avg_ratio"`
	MaxMinutes           float64 `json:"max_minutes"`
	MaxKm                float64 `json:"max_km"`
	MaxTxCount24h        float64 `json:"max_tx_count_24h"`
	MaxMerchantAvgAmount float64 `json:"max_merchant_avg_amount"`
}

func LoadNormalization(path string) (Normalization, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Normalization{}, err
	}
	var n Normalization
	if err := json.Unmarshal(b, &n); err != nil {
		return Normalization{}, err
	}
	return n, nil
}

type MCCRisk map[string]float64

func LoadMCCRisk(path string) (MCCRisk, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m MCCRisk
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
