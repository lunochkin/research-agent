package agent

import (
	"math"
	"testing"

	"github.com/lunochkin/research-agent/internal/store"
)

func TestFuseRRF(t *testing.T) {
	// chunk 2 appears in both lists -> should be boosted to the top and deduped.
	kHits := []store.Hit{
		{ChunkID: 1, PaperID: "p1"}, // rank 1
		{ChunkID: 2, PaperID: "p2"}, // rank 2
	}
	vHits := []store.Hit{
		{ChunkID: 2, PaperID: "p2"}, // rank 1
		{ChunkID: 3, PaperID: "p3"}, // rank 2
	}

	got := fuse(kHits, vHits)

	if len(got) != 3 {
		t.Fatalf("want 3 deduped chunks, got %d", len(got))
	}

	// RRF (k=60): chunk2 = 1/62 (kHits r2) + 1/61 (vHits r1) > chunk1 = 1/61 > chunk3 = 1/62.
	wantOrder := []int64{2, 1, 3}
	for i, id := range wantOrder {
		if got[i].ChunkID != id {
			t.Errorf("position %d: want chunk %d, got %d", i, id, got[i].ChunkID)
		}
	}

	wantTop := 1.0/61 + 1.0/62
	if math.Abs(got[0].Score-wantTop) > 1e-9 {
		t.Errorf("top fused score: want %v, got %v", wantTop, got[0].Score)
	}
}
