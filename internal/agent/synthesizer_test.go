package agent

import (
	"context"
	"testing"
)

func testEvidence() *Evidence {
	return &Evidence{Chunks: []RetrievedChunk{
		{ChunkID: 1, PaperID: "p1", Content: "c1"},
		{ChunkID: 2, PaperID: "p2", Content: "c2"},
	}}
}

func TestSynthesizeValidCitation(t *testing.T) {
	js := `{"text":"answer","citations":[{"paper_id":"p1","chunk_id":1}]}`
	ans, err := NewSynthesizer(fakeGen{text: js}).Synthesize(context.Background(), "q", testEvidence())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ans.Text != "answer" {
		t.Errorf("want answer text, got %q", ans.Text)
	}
}

func TestSynthesizeRejectsFabricatedCitation(t *testing.T) {
	// chunk 99 / paper p9 were never retrieved -> must be rejected.
	js := `{"text":"answer","citations":[{"paper_id":"p9","chunk_id":99}]}`
	if _, err := NewSynthesizer(fakeGen{text: js}).Synthesize(context.Background(), "q", testEvidence()); err == nil {
		t.Error("want error for citation not in evidence, got nil")
	}
}

func TestSynthesizeRejectsEmptyAnswer(t *testing.T) {
	js := `{"text":"","citations":[]}`
	if _, err := NewSynthesizer(fakeGen{text: js}).Synthesize(context.Background(), "q", testEvidence()); err == nil {
		t.Error("want error for empty answer, got nil")
	}
}

func TestSynthesizeRejectsBadJSON(t *testing.T) {
	if _, err := NewSynthesizer(fakeGen{text: "not json"}).Synthesize(context.Background(), "q", testEvidence()); err == nil {
		t.Error("want error for malformed JSON, got nil")
	}
}

func TestSynthesizeRejectsNilEvidence(t *testing.T) {
	if _, err := NewSynthesizer(fakeGen{}).Synthesize(context.Background(), "q", nil); err == nil {
		t.Error("want error for nil evidence, got nil")
	}
}
