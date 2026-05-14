package reference

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const VectorDim = 14

type Record struct {
	Vector [VectorDim]float64
	Fraud  bool
}

type rawRecord struct {
	Vector []float64 `json:"vector"`
	Label  string    `json:"label"`
}

func LoadGZIP(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var r io.Reader = f
	if isGzip(path) {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return nil, ErrInvalidReferencesJSON
	}

	var out []Record
	for dec.More() {
		var raw rawRecord
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if len(raw.Vector) != VectorDim {
			return nil, fmt.Errorf("references: dimensão %d, esperado %d", len(raw.Vector), VectorDim)
		}
		var v [VectorDim]float64
		copy(v[:], raw.Vector)
		out = append(out, Record{Vector: v, Fraud: raw.Label == "fraud"})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return out, nil
}

func isGzip(path string) bool {
	return len(path) > 3 && path[len(path)-3:] == ".gz"
}

var ErrInvalidReferencesJSON = errors.New("references: esperado '[' no início do JSON")
