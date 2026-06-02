package main

import "testing"

func BenchmarkGoSlices(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = runGo(chunkCount, vectorLen)
	}
}

func BenchmarkBallocMmap(b *testing.B) {
	if _, err := runBalloc(chunkCount, vectorLen); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := runBalloc(chunkCount, vectorLen); err != nil {
			b.Fatal(err)
		}
	}
}
