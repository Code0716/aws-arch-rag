// eval は評価ケースに対する recall@k と、（-generate 指定時は）出力品質を測り、Markdown の表で出力する。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Code0716/aws-arch-rag/backend/internal/config"
	"github.com/Code0716/aws-arch-rag/backend/internal/embed"
	"github.com/Code0716/aws-arch-rag/backend/internal/evaluate"
	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/obs"
	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	casesPath := flag.String("cases", "../data/eval/cases.yaml", "評価ケースの YAML")
	k := flag.Int("k", 5, "recall@k の k")
	label := flag.String("label", "baseline", "この実行のラベル（例: chunk512-overlap64）。改善前後の比較用")
	doGenerate := flag.Bool("generate", false, "生成も実行して出力品質を評価する（Claude の課金が発生する）")
	flag.Parse()

	cases, err := evaluate.LoadCases(*casesPath)
	if err != nil {
		return err
	}

	var cfg config.Config
	if *doGenerate {
		cfg, err = config.LoadServer()
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	log := obs.NewLogger()
	retriever := &retrieve.Retriever{
		Embed: embed.NewVoyage(cfg.VoyageAPIKey, cfg.EmbedModel, cfg.EmbedDim),
		Store: st,
		Log:   log,
	}
	var gen *generate.Generator
	if *doGenerate {
		gen = &generate.Generator{LLM: generate.NewAnthropic(cfg.AnthropicAPIKey, cfg.ClaudeModel), Log: log}
	}

	results := make([]evaluate.CaseResult, 0, len(cases))
	for _, c := range cases {
		res, err := retriever.Search(ctx, c.Requirements, *k)
		if err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}
		recall, found, total := evaluate.RecallAtK(c.Expect, res.Hits)
		r := evaluate.CaseResult{ID: c.ID, Recall: recall, Found: found, Total: total}

		if gen != nil {
			r.Generated = true
			p, usage, err := gen.Generate(ctx, c.Requirements, res.Hits, nil)
			if err != nil {
				r.GenerateError = err.Error()
			} else {
				r.SchemaProblems = evaluate.CheckSchema(p)
				r.MermaidProblems = evaluate.CheckMermaid(p.Mermaid)
				r.DroppedIDs = len(usage.DroppedIDs)
			}
		}
		results = append(results, r)
	}

	evaluate.WriteMarkdown(os.Stdout, *label, *k, results)
	return nil
}
