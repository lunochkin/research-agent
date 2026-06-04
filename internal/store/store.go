// Package store wraps Postgres (pgx) + pgvector/FTS access.
//
// It provides the two retrieval primitives — VectorSearch and KeywordSearch —
// plus corpus writes and full request logging. It returns the two ranked lists
// unmerged; hybrid fusion (RRF / score-merge) is done in the retriever.
package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Hit is one retrieved chunk with its source paper, for citations.
type Hit struct {
	ChunkID int64
	PaperID string
	Title   string
	Content string
	Score   float64 // cosine similarity (vector) or ts_rank (keyword)
}

// Filter is optional pre-filtering applied to both retrieval primitives.
type Filter struct {
	Categories []string // match if chunk's paper has ANY of these categories
}

// --- Corpus writes --------------------------------------------------------

type Paper struct {
	ID         string
	Title      string
	Abstract   string
	Authors    []string
	Categories []string
	Published  *string // ISO date or nil
}

func (s *Store) UpsertPaper(ctx context.Context, p Paper) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO papers (id, title, abstract, authors, categories, published)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			title=EXCLUDED.title, abstract=EXCLUDED.abstract,
			authors=EXCLUDED.authors, categories=EXCLUDED.categories,
			published=EXCLUDED.published`,
		p.ID, p.Title, p.Abstract, p.Authors, p.Categories, p.Published)
	return err
}

// ExistingIDs returns the subset of ids already present in papers. Used by
// ingest to skip re-embedding papers already in the corpus.
func (s *Store) ExistingIDs(ctx context.Context, ids []string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM papers WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = struct{}{}
	}
	return set, rows.Err()
}

func (s *Store) UpsertChunk(ctx context.Context, paperID string, seq int, content string, embedding []float32) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chunks (paper_id, seq, content, embedding)
		VALUES ($1,$2,$3,$4::vector)
		ON CONFLICT (paper_id, seq) DO UPDATE SET
			content=EXCLUDED.content, embedding=EXCLUDED.embedding`,
		paperID, seq, content, vecLiteral(embedding))
	return err
}

// --- Retrieval primitives -------------------------------------------------

// VectorSearch returns the k nearest chunks by cosine distance.
func (s *Store) VectorSearch(ctx context.Context, queryEmb []float32, k int, f Filter) ([]Hit, error) {
	where, args := filterClause(f, 2) // $1 reserved for the vector
	q := `
		SELECT c.id, c.paper_id, p.title, c.content,
		       1 - (c.embedding <=> $1::vector) AS score
		FROM chunks c JOIN papers p ON p.id = c.paper_id
		WHERE c.embedding IS NOT NULL ` + where + `
		ORDER BY c.embedding <=> $1::vector
		LIMIT ` + strconv.Itoa(k)
	return s.queryHits(ctx, q, append([]any{vecLiteral(queryEmb)}, args...)...)
}

// KeywordSearch returns the top-k chunks by full-text rank (BM25-ish).
func (s *Store) KeywordSearch(ctx context.Context, query string, k int, f Filter) ([]Hit, error) {
	where, args := filterClause(f, 2) // $1 reserved for the tsquery
	q := `
		SELECT c.id, c.paper_id, p.title, c.content,
		       ts_rank(c.tsv, plainto_tsquery('english', $1)) AS score
		FROM chunks c JOIN papers p ON p.id = c.paper_id
		WHERE c.tsv @@ plainto_tsquery('english', $1) ` + where + `
		ORDER BY score DESC
		LIMIT ` + strconv.Itoa(k)
	return s.queryHits(ctx, q, append([]any{query}, args...)...)
}

func (s *Store) queryHits(ctx context.Context, q string, args ...any) ([]Hit, error) {
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ChunkID, &h.PaperID, &h.Title, &h.Content, &h.Score); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// --- Full request logging -------------------------------------------------

func (s *Store) CreateRun(ctx context.Context, question string, config []byte) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO runs (question, config) VALUES ($1,$2) RETURNING id`,
		question, config).Scan(&id)
	return id, err
}

// Step is one agent-action boundary (planner, retriever[i], synthesizer, critic).
// Raw model-call detail lives in LlmCall, keyed by the step id.
type Step struct {
	RunID int64
	Agent string
	Round int
	Input []byte // JSON
}

// CreateStep inserts a step at its start (created_at marks the start time) and
// returns its id. LlmCalls and FinishStep reference that id; the row must exist
// first because llm_calls.step_id is a FK.
func (s *Store) CreateStep(ctx context.Context, st Step) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO run_steps (run_id, agent, round, input)
		VALUES ($1,$2,$3,$4)
		RETURNING id`,
		st.RunID, st.Agent, st.Round, st.Input,
	).Scan(&id)
	return id, err
}

// FinishStep closes a step: records its validated output, stamps finished_at, and
// stores errMsg if the step failed (empty errMsg -> NULL). Call once per step.
func (s *Store) FinishStep(ctx context.Context, stepID int64, output []byte, errMsg string) error {
	var errText *string
	if errMsg != "" {
		errText = &errMsg
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE run_steps SET output=$2, finished_at=now(), error=$3 WHERE id=$1`,
		stepID, output, errText)
	return err
}

// LlmCall is one raw model call under a step. Many per step on retry/re-prompt;
// carries the reproducibility detail (prompt, raw response, tokens, cost).
type LlmCall struct {
	StepID    int64
	Model     string
	Prompt    string
	Raw       string // raw model text before validation
	TokensIn  int
	TokensOut int
	CostUSD   float64
	LatencyMS int
	Attempt   int // 1-based; >1 = retry (e.g. after 429)
}

// LogLlmCall records one model call under an existing step.
func (s *Store) LogLlmCall(ctx context.Context, c LlmCall) error {
	if c.Attempt < 1 {
		c.Attempt = 1
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO llm_calls
			(step_id, model, prompt, raw, tokens_in, tokens_out, cost_usd, latency_ms, attempt)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		c.StepID, c.Model, c.Prompt, c.Raw,
		c.TokensIn, c.TokensOut, c.CostUSD, c.LatencyMS, c.Attempt)
	return err
}

func (s *Store) RunCost(ctx context.Context, runID int64) (float64, error) {
	var c float64
	err := s.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(cost_usd), 0) FROM llm_calls
			WHERE step_id IN (SELECT id FROM run_steps WHERE run_id=$1)`,
		runID).Scan(&c)
	return c, err
}

func (s *Store) FinishRun(ctx context.Context, runID int64, answer string, costUSD float64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET answer=$2, cost_usd=$3, finished_at=now() WHERE id=$1`,
		runID, answer, costUSD)
	return err
}

func (s *Store) SaveEval(ctx context.Context, runID int64, thumb int, comment string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO evals (run_id, thumb, comment) VALUES ($1,$2,$3)`,
		runID, thumb, comment)
	return err
}

// --- helpers --------------------------------------------------------------

// vecLiteral formats a []float32 as a pgvector text literal: [1,2,3].
func vecLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// filterClause builds an optional " AND ..." clause starting at $startIdx.
func filterClause(f Filter, startIdx int) (string, []any) {
	if len(f.Categories) == 0 {
		return "", nil
	}
	return fmt.Sprintf(" AND p.categories && $%d", startIdx), []any{f.Categories}
}
