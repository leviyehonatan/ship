package dockerfile

import (
	"os"
	"testing"
)

func TestParsePostgres(t *testing.T) {
	df := `FROM node:22-alpine
RUN apk add --no-cache postgresql redis
CMD ["node", "app.js"]
`
	os.WriteFile("/tmp/test_df", []byte(df), 0644)
	defer os.Remove("/tmp/test_df")

	hints := Parse("/tmp/test_df")
	found := map[string]bool{}
	for _, h := range hints {
		found[h.Name] = true
	}
	if !found["postgres"] {
		t.Error("postgres not detected")
	}
	if !found["redis"] {
		t.Error("redis not detected")
	}
}

func TestParseRedisURLExcluded(t *testing.T) {
	df := `RUN echo 'REDIS_URL=redis://localhost:6379' >> .env
CMD ["node", "app.js"]
`
	os.WriteFile("/tmp/test_df2", []byte(df), 0644)
	defer os.Remove("/tmp/test_df2")

	hints := Parse("/tmp/test_df2")
	for _, h := range hints {
		if h.Name == "redis" {
			t.Error("redis falsely detected from REDIS_URL env var")
		}
	}
}

func TestParseNone(t *testing.T) {
	df := `FROM node:22
CMD ["node", "app.js"]
`
	os.WriteFile("/tmp/test_df3", []byte(df), 0644)
	defer os.Remove("/tmp/test_df3")

	hints := Parse("/tmp/test_df3")
	if len(hints) > 0 {
		t.Errorf("expected no hints, got %v", hints)
	}
}

func TestDeduplicate(t *testing.T) {
	hints := []ServiceHint{
		{Name: "postgres"}, {Name: "postgres"}, {Name: "redis"},
	}
	result := deduplicate(hints)
	if len(result) != 2 {
		t.Errorf("expected 2 after dedup, got %d", len(result))
	}
}
