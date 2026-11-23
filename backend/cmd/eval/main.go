// eval は評価ケースに対する recall@k と、（-generate 指定時は）出力品質を測り、Markdown の表で出力する。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"

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
	kFlag := flag.String("k", "5", "recall@k の k。カンマ区切りで複数指定できる（例: 3,5,10）")
	label := flag.String("label", "baseline", "この実行のラベル（例: chunk512-overlap64）。改善前後の比較用")
	doGenerate := flag.Bool("generate", false, "生成も実行して出力品質を評価する（Claude の課金が発生する）")
	flag.Parse()

	ks, err := parseKs(*kFlag)
	if err != nil {
		return err
	}
	maxK := ks[len(ks)-1]

	cases, warnings, err := evaluate.LoadCases(*casesPath)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
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
		// 検索は最大の k で1回だけ行い、各 k は上位から切り出して採点する
		res, err := retriever.Search(ctx, c.Requirements, maxK)
		if err != nil {
			return fmt.Errorf("case %s: %w", c.ID, err)
		}
		r := evaluate.Score(c, res.Hits, ks)

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

	evaluate.WriteMarkdown(os.Stdout, *label, ks, results)
	return nil
}

// parseKs は "3,5,10" を昇順・重複なしの []int にする。
func parseKs(s string) ([]int, error) {
	var ks []int
	for _, part := range strings.Split(s, ",") {
		k, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || k <= 0 {
			return nil, fmt.Errorf("-k は正の整数のカンマ区切りで指定してください: %q", s)
		}
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return slices.Compact(ks), nil
}
