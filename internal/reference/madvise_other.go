//go:build !linux

package reference

func adviseRandom(_ []byte)   {}
func adviseWillNeed(_ []byte) {}
