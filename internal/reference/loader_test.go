package reference

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGZIP_JSONPlain(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "refs.json")
	const body = `[
	  {"vector":[0.01,0.0833,0.05,0.8261,0.1667,-1,-1,0.0432,0.25,0,1,0,0.2,0.0416],"label":"legit"},
	  {"vector":[0.5796,0.9167,1.0,0.0435,0,0.0056,0.4394,0.4598,0.4,1,0,1,0.85,0.0032],"label":"fraud"}
	]`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, err := LoadGZIP(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("len=%d", len(recs))
	}
	if recs[0].Fraud || !recs[1].Fraud {
		t.Fatalf("labels %+v", recs)
	}
}
