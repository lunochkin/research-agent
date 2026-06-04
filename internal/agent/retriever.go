package agent

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/lunochkin/research-agent/internal/llm"
	"github.com/lunochkin/research-agent/internal/store"
)

// Retriever is one worker: hybrid-retrieve for a single sub-query.
type Retriever struct {
	store *store.Store
	embed llm.Embedder
}

func NewRetriever(s *store.Store, e llm.Embedder) *Retriever {
	return &Retriever{store: s, embed: e}
}

const topN = 10

// Retrieve runs hybrid retrieval for one sub-query and returns fused evidence:
//   - embed the sub-query (embed.Embed)
//   - call store.VectorSearch and store.KeywordSearch (the two primitives)
//   - fuse the two ranked lists with RRF into one ranking
//   - map store.Hit -> RetrievedChunk
//
// The store returns the two ranked lists unmerged; fusion happens here.
func (r *Retriever) Retrieve(ctx context.Context, sq SubQuery) (*Evidence, error) {
	vectors, err := r.embed.Embed(ctx, []string{sq.Query})
	if err != nil {
		return nil, err
	}

	vHits, err := r.store.VectorSearch(ctx, vectors[0], topN, store.Filter{
		Categories: sq.Filters.Categories,
	})
	if err != nil {
		return nil, err
	}

	kHits, err := r.store.KeywordSearch(ctx, sq.Query, topN, store.Filter{
		Categories: sq.Filters.Categories,
	})
	if err != nil {
		return nil, err
	}
	chunks := fuse(kHits, vHits)

	return &Evidence{Chunks: chunks}, nil
}

const k = 60
const N = 5

type fused struct {
	hit   store.Hit
	score float64
}

func fuse(kHits []store.Hit, vHits []store.Hit) []RetrievedChunk {
	acc := map[int64]*fused{} // key = ChunkID

	addList := func(hits []store.Hit) {
		for i, h := range hits {
			f := acc[h.ChunkID]
			if f == nil {
				f = &fused{hit: h}
				acc[h.ChunkID] = f
			}
			f.score += 1.0 / float64(k+i+1)
		}
	}
	addList(kHits)
	addList(vHits)

	fusedList := slices.Collect(maps.Values(acc))

	slices.SortFunc(fusedList, func(a, b *fused) int {
		// b, a = descending
		if c := cmp.Compare(b.score, a.score); c != 0 {
			return c
		}
		return cmp.Compare(a.hit.ChunkID, b.hit.ChunkID)
	})

	if len(fusedList) > N {
		fusedList = fusedList[:N]
	}

	chunks := make([]RetrievedChunk, len(fusedList))
	for i, f := range fusedList {
		chunks[i] = RetrievedChunk{
			ChunkID: f.hit.ChunkID,
			PaperID: f.hit.PaperID,
			Title:   f.hit.Title,
			Content: f.hit.Content,
			Score:   f.score,
		}
	}
	return chunks
}
