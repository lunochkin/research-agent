// Package ingest loads a Kaggle arXiv metadata dump into the corpus:
// fetch(file) -> filter by topic -> chunk (abstracts-first) -> embed -> index.
package ingest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/llm"
	"github.com/lunochkin/research-agent/internal/store"
)

const (
	embedBatch   = 512 // papers per embedding API call (OpenAI cap: 2048 / 300k tok)
	embedWorkers = 4   // concurrent embed+persist workers (4×512×~250tok < 1M TPM)
)

// batch is a unit of work handed from the producer to an embed worker.
type batch struct {
	papers []store.Paper
	texts  []string
}

type Ingester struct {
	store *store.Store
	embed llm.Embedder
	topic config.Topic
	force bool // re-embed papers already in the corpus (default: skip them)
}

func New(s *store.Store, e llm.Embedder, t config.Topic, force bool) *Ingester {
	return &Ingester{store: s, embed: e, topic: t, force: force}
}

// rawPaper mirrors the Kaggle arXiv snapshot JSON-lines schema (subset).
type rawPaper struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Abstract   string `json:"abstract"`
	Authors    string `json:"authors"`
	Categories string `json:"categories"` // space-separated
	UpdateDate string `json:"update_date"`
}

// Run streams the dump at path, ingesting papers that match the configured topic.
//
// Pipeline: a single producer goroutine scans + filters + batches the file; a
// pool of workers embeds and persists batches concurrently. Embedding is the
// I/O-bound bottleneck, so parallel workers give near-linear speedup.
func (in *Ingester) Run(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var seen, kept, skipped, embedded atomic.Int64

	g, ctx := errgroup.WithContext(ctx)
	batches := make(chan batch, embedWorkers*2)

	// Workers: embed + persist each batch concurrently.
	for range embedWorkers {
		g.Go(func() error {
			for b := range batches {
				if err := in.process(ctx, b, &seen, &skipped, &embedded); err != nil {
					return err
				}
			}
			return nil
		})
	}

	// Producer: scan, filter, batch, dispatch. Closes the channel when done.
	g.Go(func() error {
		defer close(batches)
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024) // abstracts can be long

		var papers []store.Paper
		var texts []string
		dispatch := func() error {
			if len(papers) == 0 {
				return nil
			}
			select {
			case batches <- batch{papers: papers, texts: texts}:
			case <-ctx.Done():
				return ctx.Err()
			}
			papers, texts = nil, nil // ownership passed to the worker; fresh slices
			return nil
		}

		for sc.Scan() {
			seen.Add(1)
			var r rawPaper
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				slog.Warn("skip malformed line", "err", err)
				continue
			}
			if !in.matches(r) {
				continue
			}
			papers = append(papers, store.Paper{
				ID:         r.ID,
				Title:      clean(r.Title),
				Abstract:   clean(r.Abstract),
				Authors:    []string{strings.TrimSpace(r.Authors)},
				Categories: strings.Fields(r.Categories),
				Published:  normalizeDate(r.UpdateDate),
			})
			// abstracts-first: one chunk per paper (title + abstract for retrieval).
			texts = append(texts, clean(r.Title)+"\n"+clean(r.Abstract))
			kept.Add(1)
			if len(papers) >= embedBatch {
				if err := dispatch(); err != nil {
					return err
				}
			}
		}
		if err := sc.Err(); err != nil {
			return err
		}
		return dispatch()
	})

	if err := g.Wait(); err != nil {
		return err
	}
	slog.Info("ingest done",
		"seen", seen.Load(), "matched", kept.Load(),
		"skipped_existing", skipped.Load(), "embedded", embedded.Load())
	return nil
}

// process skip-checks, embeds, and persists one batch. Safe for concurrent use:
// the pgx pool and the HTTP embed client are both goroutine-safe.
func (in *Ingester) process(ctx context.Context, b batch, seen, skipped, embedded *atomic.Int64) error {
	papers, texts := b.papers, b.texts

	// Default: skip papers already in the corpus so we don't re-embed (cost).
	if !in.force {
		ids := make([]string, len(papers))
		for i, p := range papers {
			ids[i] = p.ID
		}
		existing, err := in.store.ExistingIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("existing ids: %w", err)
		}
		if len(existing) > 0 {
			np := make([]store.Paper, 0, len(papers))
			nt := make([]string, 0, len(texts))
			for i, p := range papers {
				if _, ok := existing[p.ID]; ok {
					skipped.Add(1)
					continue
				}
				np = append(np, p)
				nt = append(nt, texts[i])
			}
			papers, texts = np, nt
		}
	}
	if len(papers) == 0 {
		// Whole batch already in corpus (resume): no embed, but show movement.
		slog.Info("ingest progress",
			"seen", seen.Load(), "embedded", embedded.Load(), "skipped_existing", skipped.Load())
		return nil
	}

	vecs, err := in.embed.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed batch: %w", err)
	}
	for i, p := range papers {
		if err := in.store.UpsertPaper(ctx, p); err != nil {
			return fmt.Errorf("upsert paper %s: %w", p.ID, err)
		}
		if err := in.store.UpsertChunk(ctx, p.ID, 0, texts[i], vecs[i]); err != nil {
			return fmt.Errorf("upsert chunk %s: %w", p.ID, err)
		}
	}
	slog.Info("ingest progress",
		"seen", seen.Load(), "embedded", embedded.Add(int64(len(papers))), "skipped_existing", skipped.Load())
	return nil
}

// matches keeps a paper if it shares a configured category AND contains a keyword.
func (in *Ingester) matches(r rawPaper) bool {
	cats := strings.Fields(r.Categories)
	if !anyOverlap(cats, in.topic.Categories) {
		return false
	}
	if len(in.topic.Keywords) == 0 {
		return true
	}
	hay := strings.ToLower(r.Title + " " + r.Abstract)
	for _, kw := range in.topic.Keywords {
		if strings.Contains(hay, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func anyOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(b))
	for _, x := range b {
		set[x] = struct{}{}
	}
	for _, x := range a {
		if _, ok := set[x]; ok {
			return true
		}
	}
	return false
}

func clean(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\n", " ")) }

func normalizeDate(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
