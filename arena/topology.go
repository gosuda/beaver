package arena

import "math/bits"

type topology struct {
	scale  int
	poly   uint64
	matrix [][]uint64
}

func newTopology(scale int, poly uint64) *topology {
	return &topology{
		scale:  scale,
		poly:   poly,
		matrix: buildPandiagonal(scale),
	}
}

func buildPandiagonal(scale int) [][]uint64 {
	matrix := make([][]uint64, scale)
	for row := 0; row < scale; row++ {
		matrix[row] = make([]uint64, scale)
		for col := 0; col < scale; col++ {
			matrix[row][col] = uint64((row+col)%scale + 1)
		}
	}
	return matrix
}

func (t *topology) bucket(threadID, chunkID uint64) int {
	row := int(threadID % uint64(t.scale))
	col := int(chunkID % uint64(t.scale))
	factorMagic := t.matrix[row][col]
	factorOrtho := threadID ^ chunkID
	return int(gfMultiply(factorMagic, factorOrtho, t.poly, t.scale) % uint64(t.scale))
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
