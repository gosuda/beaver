package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	if _, err := runBalloc(chunkCount, vectorLen); err != nil {
		log.Fatal(err)
	}

	run("go", func() (float64, error) {
		return runGo(chunkCount, vectorLen), nil
	})
	run("balloc", func() (float64, error) {
		return runBalloc(chunkCount, vectorLen)
	})
}

func run(name string, fn func() (float64, error)) {
	start := time.Now()
	sum, err := fn()
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	fmt.Printf("%-6s sum=%0.4f elapsed=%s\n", name, sum, time.Since(start))
}
