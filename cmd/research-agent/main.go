// Command research-agent is the CLI entrypoint: ingest a corpus, ask a question,
// record an eval. Wiring only — agent logic lives in internal/agent.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lunochkin/research-agent/internal/agent"
	"github.com/lunochkin/research-agent/internal/config"
	"github.com/lunochkin/research-agent/internal/ingest"
	"github.com/lunochkin/research-agent/internal/llm"
	"github.com/lunochkin/research-agent/internal/migrate"
	"github.com/lunochkin/research-agent/internal/store"
)

func main() {
	loadDotEnv(".env")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel()})))

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}

// logLevel reads LOG_LEVEL (debug|info|warn|error) from the environment.
// Defaults to info, so slog.Debug lines stay silent unless LOG_LEVEL=debug.
func logLevel() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func run(cmd string, args []string) error {
	ctx := context.Background()

	// migrate only needs the DB URL, not the full config (LLM keys etc.) — handle
	// it before config.Load so the schema can be applied on a bare checkout.
	if cmd == "migrate" {
		return cmdMigrate(args)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch cmd {
	case "ingest":
		return cmdIngest(ctx, cfg, args)
	case "ask":
		return cmdAsk(ctx, cfg, args)
	case "eval":
		return cmdEval(ctx, cfg, args)
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	down := fs.Bool("down", false, "roll back all migrations instead of applying them")
	_ = fs.Parse(args)

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://rag:rag@localhost:5432/rag?sslmode=disable"
	}
	if *down {
		if err := migrate.Down(url); err != nil {
			return err
		}
		slog.Info("migrations rolled back")
		return nil
	}
	if err := migrate.Up(url); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}

func cmdIngest(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	file := fs.String("file", "", "path to Kaggle arXiv JSON-lines dump")
	force := fs.Bool("force", false, "re-embed papers already in the corpus (default: skip them)")
	_ = fs.Parse(args)
	if *file == "" {
		return fmt.Errorf("ingest: --file required")
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	emb, err := llm.NewEmbedder(cfg.Embed)
	if err != nil {
		return err
	}
	return ingest.New(st, emb, cfg.Topic, *force).Run(ctx, *file)
}

func cmdAsk(ctx context.Context, cfg *config.Config, args []string) error {
	question := strings.TrimSpace(strings.Join(args, " "))
	if question == "" {
		return fmt.Errorf("ask: question required (usage: research-agent ask \"...\")")
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	gen, err := llm.NewGenerator(cfg.Gen)
	if err != nil {
		return err
	}
	emb, err := llm.NewEmbedder(cfg.Embed)
	if err != nil {
		return err
	}

	orch := agent.NewOrchestrator(
		agent.NewPlanner(gen, cfg.Topic),
		agent.NewRetriever(st, emb),
		agent.NewSynthesizer(gen),
		agent.NewCritic(gen, cfg.Topic),
		cfg,
		st,
	)

	res, err := orch.Run(ctx, question)
	if err != nil {
		return err
	}
	fmt.Println(res.Answer.Text)
	fmt.Println("\nCitations:")
	for _, c := range res.Answer.Citations {
		fmt.Printf("  - %s (chunk %d)\n", c.PaperID, c.ChunkID)
	}
	fmt.Printf("\nrun %d · %d round(s) · $%.4f\n", res.RunID, res.Rounds, res.CostUSD)
	fmt.Printf("rate it: research-agent eval --run %d --thumb +1 --comment \"...\"\n", res.RunID)
	return nil
}

func cmdEval(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	runID := fs.Int64("run", 0, "run id to rate")
	thumb := fs.Int("thumb", 0, "+1 or -1")
	comment := fs.String("comment", "", "free-text comment")
	_ = fs.Parse(args)
	if *runID == 0 || (*thumb != 1 && *thumb != -1) {
		return fmt.Errorf("eval: --run <id> and --thumb <+1|-1> required")
	}

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.SaveEval(ctx, *runID, *thumb, *comment); err != nil {
		return err
	}
	slog.Info("eval saved", "run", *runID, "thumb", *thumb)
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `research-agent — multi-agent RAG over arXiv

usage:
  research-agent migrate [--down]
  research-agent ingest --file <dump.json>
  research-agent ask "<question>"
  research-agent eval --run <id> --thumb <+1|-1> [--comment "..."]
`)
}

// loadDotEnv loads KEY=VALUE lines from path into the process env (best-effort).
// Existing env vars win. Avoids a godotenv dependency.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	if err := sc.Err(); err != nil {
		slog.Warn("loadDotEnv: scan failed", "path", path, "err", err)
	}
}
