package balloc

import "math/bits"

func buildPandiagonal(scale int) [][]uint32 {
	matrix := make([][]uint32, scale)
	for row := 0; row < scale; row++ {
		matrix[row] = make([]uint32, scale)
		for col := 0; col < scale; col++ {
			matrix[row][col] = uint32((row+col)%scale + 1)
		}
	}
	return matrix
}

func bucketFor(matrix [][]uint32, scale int, threadID, chunkID uint64) int {
	row := int(threadID % uint64(scale))
	col := int(chunkID % uint64(scale))
	factorMagic := uint64(matrix[row][col])
	factorOrtho := threadID ^ chunkID
	return int(gfMultiply(factorMagic, factorOrtho, GFPoly, scale) % uint64(scale))
}

func gfMultiply(a, b, poly uint64, scale int) uint64 {
	degree := bits.Len(uint(scale - 1))
	mask := uint64(scale - 1)
	var out uint64
	for b != 0 {
		if b&1 == 1 {
			out ^= a
		}
		b >>= 1
		a <<= 1
		if a&uint64(scale) != 0 {
			a ^= poly
		}
		a &= (uint64(1) << degree) - 1
	}
	return out & mask
}
