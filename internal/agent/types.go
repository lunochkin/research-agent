// Package agent holds the multi-agent topology: the structured-output contracts
// (this file) plus the planner, retriever, synthesizer, critic, and the
// orchestrator that wires them.
package agent

import "errors"

// ErrNotImplemented is returned by agent methods that are not yet implemented.
var ErrNotImplemented = errors.New("not implemented")

// Filters pre-narrow retrieval (planner emits them, retriever applies them).
type Filters struct {
	Categories []string `json:"categories,omitempty"`
}

// SubQuery is one decomposed search the planner hands to a retriever worker.
type SubQuery struct {
	Query   string  `json:"query"`
	Filters Filters `json:"filters,omitzero"`
}

// Plan is the planner's validated output.
type Plan struct {
	SubQueries []SubQuery `json:"sub_queries"`
}

// RetrievedChunk is one piece of evidence with enough provenance to cite it.
type RetrievedChunk struct {
	ChunkID int64   `json:"chunk_id"`
	PaperID string  `json:"paper_id"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"` // post-fusion score
}

// Evidence is the fused, ranked chunk set for a sub-query (or merged across all).
type Evidence struct {
	Chunks []RetrievedChunk `json:"chunks"`
}

// Citation ties a claim in the answer back to a retrieved chunk.
type Citation struct {
	PaperID string `json:"paper_id"`
	ChunkID int64  `json:"chunk_id"`
}

// Answer is the synthesizer's validated output.
type Answer struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations"`
}

// Critique is the critic's validated output: grounding verdict + gaps that, if
// present, drive a bounded re-retrieval round.
type Critique struct {
	Grounded        bool       `json:"grounded"`
	Gaps            []string   `json:"gaps,omitempty"`
	FollowupQueries []SubQuery `json:"followup_queries,omitempty"`
}
