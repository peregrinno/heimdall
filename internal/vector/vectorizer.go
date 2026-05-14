package vector

import (
	"math"
	"time"

	"heimdall/internal/config"
	"heimdall/internal/domain"
)

const (
	dim        = 14
	defaultMCC = 0.5
)

func Limitar(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func SpecWeekday(t time.Time) int {
	w := int(t.UTC().Weekday())
	return (w + 6) % 7
}

func Build(req domain.FraudScoreRequest, n config.Normalization, mcc config.MCCRisk) [dim]float64 {
	var v [dim]float64
	tx := req.Transaction
	cu := req.Customer
	me := req.Merchant
	te := req.Terminal

	v[0] = Limitar(tx.Amount / n.MaxAmount)
	v[1] = Limitar(float64(tx.Installments) / n.MaxInstallments)

	avg := cu.AvgAmount
	if avg <= 0 || math.IsNaN(avg) || math.IsInf(avg, 0) {
		v[2] = 1
	} else {
		v[2] = Limitar((tx.Amount / avg) / n.AmountVsAvgRatio)
	}

	rt := tx.RequestedAt.UTC()
	v[3] = Limitar(float64(rt.Hour()) / 23)
	v[4] = Limitar(float64(SpecWeekday(rt)) / 6)

	if req.LastTransaction == nil {
		v[5] = -1
		v[6] = -1
	} else {
		minutes := rt.Sub(req.LastTransaction.Timestamp.UTC()).Minutes()
		if minutes < 0 {
			minutes = 0
		}
		v[5] = Limitar(minutes / n.MaxMinutes)
		v[6] = Limitar(req.LastTransaction.KmFromCurrent / n.MaxKm)
	}

	v[7] = Limitar(te.KmFromHome / n.MaxKm)
	v[8] = Limitar(float64(cu.TxCount24h) / n.MaxTxCount24h)

	if te.IsOnline {
		v[9] = 1
	}
	if te.CardPresent {
		v[10] = 1
	}

	if !merchantKnown(me.ID, cu.KnownMerchants) {
		v[11] = 1
	}

	risk, ok := mcc[me.MCC]
	if !ok {
		risk = defaultMCC
	}
	v[12] = risk
	v[13] = Limitar(me.AvgAmount / n.MaxMerchantAvgAmount)

	return v
}

func merchantKnown(id string, known []string) bool {
	for _, k := range known {
		if k == id {
			return true
		}
	}
	return false
}
