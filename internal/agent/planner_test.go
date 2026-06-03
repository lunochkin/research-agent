package agent

import (
	"context"
	"testing"

	"github.com/lunochkin/research-agent/internal/config"
)

func newTestPlanner(text string) *Planner {
	return NewPlanner(fakeGen{text: text}, config.Topic{Categories: []string{"cs.AI", "cs.LG"}})
}

func TestPlanValid(t *testing.T) {
	js := `{"sub_queries":[
		{"query":"a","filters":{"categories":["cs.AI"]}},
		{"query":"b","filters":{"categories":["cs.LG"]}}
	]}`
	p, err := newTestPlanner(js).Plan(context.Background(), "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.SubQueries) != 2 {
		t.Fatalf("want 2 sub-queries, got %d", len(p.SubQueries))
	}
}

func TestPlanRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"too few":        `{"sub_queries":[{"query":"a","filters":{"categories":["cs.AI"]}}]}`,
		"too many":       `{"sub_queries":[{"query":"a"},{"query":"b"},{"query":"c"},{"query":"d"},{"query":"e"}]}`,
		"empty query":    `{"sub_queries":[{"query":"a"},{"query":"  "}]}`,
		"bad category":   `{"sub_queries":[{"query":"a","filters":{"categories":["cs.AI"]}},{"query":"b","filters":{"categories":["nope"]}}]}`,
		"malformed json": `not json`,
	}
	for name, js := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := newTestPlanner(js).Plan(context.Background(), "q"); err == nil {
				t.Errorf("want error, got nil")
			}
		})
	}
}
