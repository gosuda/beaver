package arena

import "testing"

func TestDiscoverArenaScale(t *testing.T) {
	tests := map[int]int{
		1:  8,
		8:  8,
		9:  16,
		16: 16,
		17: 32,
	}
	for cores, want := range tests {
		if got := discoverArenaScale(cores); got != want {
			t.Fatalf("discoverArenaScale(%d) = %d, want %d", cores, got, want)
		}
	}
}

func TestPolynomialForScale(t *testing.T) {
	if got := polynomialForScale(8); got != 0x11B {
		t.Fatalf("polynomialForScale(8) = %#x", got)
	}
	if got := polynomialForScale(16); got != 0x1002D {
		t.Fatalf("polynomialForScale(16) = %#x", got)
	}
	if got := polynomialForScale(32); got != 0x40000007 {
		t.Fatalf("polynomialForScale(32) = %#x", got)
	}
}

func TestTopologyBucketBoundary(t *testing.T) {
	topo := newTopology(8, 0x11B)
	for threadID := uint64(0); threadID < 64; threadID++ {
		for chunkID := uint64(0); chunkID < 64; chunkID++ {
			got := topo.bucket(threadID, chunkID)
			if got < 0 || got >= 8 {
				t.Fatalf("bucket(%d, %d) = %d, outside scale", threadID, chunkID, got)
			}
		}
	}
}

func TestMemTreeAggregatesAndClearsOwners(t *testing.T) {
	var tree memTree
	tree.insert(100, 64, 0b001)
	tree.insert(100, 64, 0b100)
	node := tree.find(100)
	if node == nil || node.OwnerMask != 0b101 {
		t.Fatalf("owner mask = %03b, want 101", node.OwnerMask)
	}
	remaining, found := tree.freeVarying(100, 0b001)
	if !found || remaining != 0b100 {
		t.Fatalf("remaining = %03b, found = %v; want 100, true", remaining, found)
	}
	remaining, found = tree.freeVarying(100, 0b100)
	if !found || remaining != 0 {
		t.Fatalf("remaining = %03b, found = %v; want 0, true", remaining, found)
	}
}

func TestArenaLifecycle(t *testing.T) {
	a := New()
	defer a.Close()

	addr, bucket, err := a.Alloc(128, 0b001, 3, 9)
	if err != nil {
		t.Fatal(err)
	}
	if bucket < 0 || bucket >= ArenaScale {
		t.Fatalf("bucket = %d, outside ArenaScale %d", bucket, ArenaScale)
	}
	if err := a.Share(addr, 0b010); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := a.FreeVarying(addr, 0b001)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed {
		t.Fatal("first free reclaimed shared block")
	}
	reclaimed, err = a.FreeVarying(addr, 0b010)
	if err != nil {
		t.Fatal(err)
	}
	if !reclaimed {
		t.Fatal("final free did not reclaim block")
	}
}
