package balloc

import "runtime"

const minScale = 8

var (
	ArenaScale = discoverScale(runtime.GOMAXPROCS(0))
	GFPoly     = polynomialForScale(ArenaScale)
)

func discoverScale(cores int) int {
	if cores < minScale {
		return minScale
	}
	n := 1
	for n < cores {
		n <<= 1
	}
	return n
}

func polynomialForScale(scale int) uint64 {
	switch {
	case scale <= 8:
		return 0x11B
	case scale <= 16:
		return 0x1002D
	default:
		return 0x40000007
	}
}
