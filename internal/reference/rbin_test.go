package reference

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRBinAndMmapLen(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "refs.json")
	const body = `[
	  {"vector":[0.01,0.0833,0.05,0.8261,0.1667,-1,-1,0.0432,0.25,0,1,0,0.2,0.0416],"label":"legit"},
	  {"vector":[0.5796,0.9167,1.0,0.0435,0,0.0056,0.4394,0.4598,0.4,1,0,1,0.85,0.0032],"label":"fraud"}
	]`
	if err := os.WriteFile(jsonPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rbinPath := filepath.Join(dir, "refs.rbin")
	f, err := os.Open(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := BuildRBin(f, rbinPath)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("n=%d", n)
	}
	m, err := OpenMappedRBin(rbinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if m.Len() != 2 {
		t.Fatalf("mmap len=%d", m.Len())
	}
}
