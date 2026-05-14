package vector

func PartitionKey(v *[dim]float64) uint8 {
	var k uint8
	if v[9] >= 0.5 {
		k |= 1 << 0
	}
	if v[10] >= 0.5 {
		k |= 1 << 1
	}
	if v[11] >= 0.5 {
		k |= 1 << 2
	}
	if v[5] < 0 {
		k |= 1 << 3
	}
	if v[6] < 0 {
		k |= 1 << 4
	}
	return k
}
