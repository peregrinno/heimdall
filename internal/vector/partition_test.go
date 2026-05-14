package vector

import "testing"

func TestPartitionKey_sentinelBits(t *testing.T) {
	var a [dim]float64
	a[5], a[6] = -1, -1
	a[9], a[10], a[11] = 1, 0, 1
	got := PartitionKey(&a)
	want := uint8((1 << 0) | (1 << 2) | (1 << 3) | (1 << 4))
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestPartitionKey_noSentinel(t *testing.T) {
	var a [dim]float64
	a[5], a[6] = 0.1, 0.2
	got := PartitionKey(&a)
	if got&(1<<3) != 0 || got&(1<<4) != 0 {
		t.Fatalf("sentinel bits set: %d", got)
	}
}
