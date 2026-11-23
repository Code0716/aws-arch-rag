package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const voyageEndpoint = "https://api.voyageai.com/v1/embeddings"

// Voyage は Voyage AI の埋め込み API クライアント。
type Voyage struct {
	apiKey    string
	model     string
	dim       int
	batchSize int
	endpoint  string
	http      *http.Client
}

func NewVoyage(apiKey, model string, dim int) *Voyage {
	return &Voyage{
		apiKey:    apiKey,
		model:     model,
		dim:       dim,
		batchSize: 64,
		endpoint:  voyageEndpoint,
		http:      &http.Client{Timeout: 60 * time.Second},
	}
}

func (v *Voyage) Dim() int { return v.dim }

type voyageRequest struct {
	Input           []string `json:"input"`
	Model           string   `json:"model"`
	InputType       string   `json:"input_type"`
	OutputDimension int      `json:"output_dimension,omitempty"`
}

type voyageResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (v *Voyage) Embed(ctx context.Context, texts []string, kind InputKind) (Result, error) {
	res := Result{Vectors: make([][]float32, 0, len(texts))}
	for start := 0; start < len(texts); start += v.batchSize {
		end := min(start+v.batchSize, len(texts))
		part, err := v.embedBatch(ctx, texts[start:end], kind)
		if err != nil {
			return Result{}, err
		}
		res.Vectors = append(res.Vectors, part.Vectors...)
		res.Tokens += part.Tokens
	}
	return res, nil
}

func (v *Voyage) embedBatch(ctx context.Context, texts []string, kind InputKind) (Result, error) {
	body, err := json.Marshal(voyageRequest{
		Input:           texts,
		Model:           v.model,
		InputType:       string(kind),
		OutputDimension: v.dim,
	})
	if err != nil {
		return Result{}, err
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
		out, retryable, err := v.do(ctx, body, len(texts))
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retryable {
			break
		}
	}
	return Result{}, lastErr
}

func (v *Voyage) do(ctx context.Context, body []byte, n int) (Result, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.http.Do(req)
	if err != nil {
		return Result{}, true, fmt.Errorf("voyage request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return Result{}, retryable, fmt.Errorf("voyage status %d: %s", resp.StatusCode, raw)
	}

	var parsed voyageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Result{}, false, fmt.Errorf("voyage decode: %w", err)
	}
	if len(parsed.Data) != n {
		return Result{}, false, fmt.Errorf("voyage returned %d embeddings for %d inputs", len(parsed.Data), n)
	}
	vecs := make([][]float32, n)
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= n {
			return Result{}, false, fmt.Errorf("voyage returned bad index %d", d.Index)
		}
		vecs[d.Index] = d.Embedding
	}
	return Result{Vectors: vecs, Tokens: parsed.Usage.TotalTokens}, false, nil
}
