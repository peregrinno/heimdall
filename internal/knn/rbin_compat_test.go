package knn

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"heimdall/internal/reference"
)

func TestRBinKNNMatchesMemoryWhenFloat32Exact(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "refs.json")
	const body = `[
	  {"vector":[0,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"legit"},
	  {"vector":[1,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"legit"},
	  {"vector":[2,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"legit"},
	  {"vector":[3,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"legit"},
	  {"vector":[4,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"legit"},
	  {"vector":[100,0,0,0,0,-1,-1,0,0,0,0,0,0.5,0],"label":"fraud"}
	]`
	if err := os.WriteFile(jsonPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rbinPath := filepath.Join(dir, "refs.rbin")
	f, err := os.Open(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reference.BuildRBin(f, rbinPath); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	recs, err := reference.LoadGZIP(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := reference.OpenMappedRBin(rbinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()

	var q [reference.VectorDim]float64
	q[0] = 0.5
	mem := FraudFraction(&q, recs)
	rb := FraudFractionRBin(&q, m.Raw(), m.Len())
	if math.Abs(mem-rb) > 1e-9 {
		t.Fatalf("mem=%v rbin=%v", mem, rb)
	}
}
