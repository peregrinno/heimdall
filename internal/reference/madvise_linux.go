//go:build linux

package reference

import "golang.org/x/sys/unix"

func adviseRandom(b []byte) {
	if len(b) == 0 {
		return
	}
	_ = unix.Madvise(b, unix.MADV_RANDOM)
}

func adviseWillNeed(b []byte) {
	if len(b) == 0 {
		return
	}
	_ = unix.Madvise(b, unix.MADV_WILLNEED)
}
