// Package mock は API キー・DB なしで画面を確認するための、固定データを返す実装を提供する。
// server.Server のインターフェース（Retriever / Proposer / ChunkGetter）を満たす。
package mock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Code0716/aws-arch-rag/backend/internal/generate"
	"github.com/Code0716/aws-arch-rag/backend/internal/retrieve"
	"github.com/Code0716/aws-arch-rag/backend/internal/store"
)

// Chunks は固定のチャンク。内容はこのリポジトリ用に書いた説明文で、AWS の文書ではない。
var Chunks = []store.Chunk{
	{ID: 1, Source: "mock-reliability.md", Pillar: "reliability", Section: "Distribute across Availability Zones",
		Content: "Run stateless application servers in at least two Availability Zones behind a load balancer, and use a managed database with a standby in another zone, so that the loss of one zone does not take the service down."},
	{ID: 2, Source: "mock-reliability.md", Pillar: "reliability", Section: "Back up and test recovery",
		Content: "Enable automated backups and point-in-time recovery for data stores, and rehearse restores regularly. A backup that has never been restored is an assumption, not a recovery plan."},
	{ID: 3, Source: "mock-cost-optimization.md", Pillar: "cost-optimization", Section: "Match supply to demand",
		Content: "Use auto scaling and managed services so that capacity follows traffic. Serve static assets and images from a content delivery network and object storage rather than from application servers."},
	{ID: 4, Source: "mock-cost-optimization.md", Pillar: "cost-optimization", Section: "Use pricing models that fit steady load",
		Content: "For the steady baseline of a workload, commitment-based pricing lowers cost. Keep on-demand capacity for the variable part of the load."},
	{ID: 5, Source: "mock-security.md", Pillar: "security", Section: "Protect data at rest and in transit",
		Content: "Encrypt data at rest with managed keys, enforce TLS for all connections, and restrict uploaded-object access with least-privilege policies and pre-signed URLs."},
	{ID: 6, Source: "mock-performance-efficiency.md", Pillar: "performance-efficiency", Section: "Cache close to the user",
		Content: "Place a cache and a content delivery network in front of read-heavy paths so that most requests are answered without reaching the database."},
}

// Proposal は固定の構成案。chunk_ids は Chunks の ID だけを参照する。
var Proposal = generate.Proposal{
	Summary: "ALB と複数AZのECSで画面・APIを提供し、商品画像はS3とCloudFrontで配信、データはマルチAZのRDSに置く構成です。コストはAuto ScalingとCDNで抑えます。",
	Components: []generate.Component{
		{Name: "CloudFront", Role: "商品画像と静的ファイルの配信（キャッシュ）"},
		{Name: "ALB", Role: "複数AZのアプリケーションへ負荷分散"},
		{Name: "ECS Fargate", Role: "ステートレスなアプリケーション（Auto Scaling）"},
		{Name: "RDS (Multi-AZ)", Role: "注文・商品データ。スタンバイを別AZに配置"},
		{Name: "S3", Role: "商品画像のアップロード先"},
	},
	Mermaid: "flowchart LR\n  U[\"User\"] --> CF[\"CloudFront\"]\n  CF --> ALB[\"ALB\"]\n  ALB --> ECS[\"ECS Fargate (2 AZ)\"]\n  ECS --> RDS[(\"RDS Multi-AZ\")]\n  ECS --> S3[\"S3 (product images)\"]\n  CF --> S3",
	Rationale: []generate.Rationale{
		{Pillar: "reliability", Text: "アプリケーションを2つ以上のAZに分散し、RDSもマルチAZにすることで、1つのAZ障害でサービスが止まらないようにします。", ChunkIDs: []int64{1}},
		{Pillar: "reliability", Text: "自動バックアップとポイントインタイムリカバリを有効にし、リストアの手順を定期的に試します。", ChunkIDs: []int64{2}},
		{Pillar: "cost-optimization", Text: "トラフィックに合わせて自動でスケールさせ、画像はCDNとS3から配信してアプリケーションサーバーの負荷を下げます。", ChunkIDs: []int64{3, 6}},
		{Pillar: "security", Text: "画像のアップロードは署名付きURLで行い、保存データは暗号化します。", ChunkIDs: []int64{5}},
	},
	Concerns: []string{
		"マルチAZ構成はシングルAZよりコストが上がるため、予算とのバランスを確認する必要があります。",
		"画像のアップロード後の処理（リサイズ・ウイルススキャン）が要件に含まれるか不明です。",
	},
	Questions: []string{
		"可用性の目標値（例: 99.9%）と、許容できる復旧時間・復旧時点は？",
		"ピーク時のアクセス数と、セール等で急増する頻度は？",
		"月額の予算の上限は？",
	},
}

// Retriever は常に Chunks の先頭 k 件を返す。
type Retriever struct{}

func (Retriever) Search(_ context.Context, _ string, k int) (retrieve.Result, error) {
	n := min(k, len(Chunks))
	hits := make([]store.Hit, n)
	for i := 0; i < n; i++ {
		c := Chunks[i]
		hits[i] = store.Hit{ID: c.ID, Score: 0.9 - 0.05*float64(i), Source: c.Source, Pillar: c.Pillar, Section: c.Section, Content: c.Content}
	}
	return retrieve.Result{Hits: hits, EmbedTokens: 24, Duration: 12 * time.Millisecond}, nil
}

// Proposer は固定の構成案を、LLM のようにゆっくり流しながら返す。
type Proposer struct {
	// Delay は断片ごとの待ち時間。0 なら待たない（テスト用）。
	Delay time.Duration
}

func (p Proposer) Generate(ctx context.Context, _ string, _ []store.Hit, onText func(string)) (generate.Proposal, generate.Usage, error) {
	raw, _ := json.Marshal(Proposal)
	const step = 48
	runes := []rune(string(raw))
	for i := 0; i < len(runes); i += step {
		select {
		case <-ctx.Done():
			return generate.Proposal{}, generate.Usage{}, ctx.Err()
		case <-time.After(p.Delay):
		}
		onText(string(runes[i:min(i+step, len(runes))]))
	}
	return Proposal, generate.Usage{InputTokens: 2400, OutputTokens: len(runes) / 2, Model: "mock", LLMDuration: time.Duration(len(runes)/step) * p.Delay}, nil
}

// ChunkGetter は Chunks から ID で引く。
type ChunkGetter struct{}

func (ChunkGetter) Get(_ context.Context, id int64) (store.Chunk, error) {
	for _, c := range Chunks {
		if c.ID == id {
			return c, nil
		}
	}
	return store.Chunk{}, store.ErrNotFound
}
