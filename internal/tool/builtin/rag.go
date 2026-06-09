package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PeneyLove/ai-novel-matrix-studio/internal/config"
	"github.com/PeneyLove/ai-novel-matrix-studio/internal/tool"
)

func init() { tool.RegisterBuiltin(ragSearch{}) }

// SetRAGConfig stores the RAG configuration for the rag_search tool.
// Called during boot before the agent starts.
func SetRAGConfig(cfg config.RAGConfig) { ragCfg = cfg }

var ragCfg config.RAGConfig

type ragSearch struct{}

const ragTimeout = 30 * time.Second

func (ragSearch) Name() string { return "rag_search" }

func (ragSearch) Description() string {
	return "Search the novel-writing knowledge base (RAG) for reference material from popular web novels. Retrieves relevant excerpts and writing techniques matching the query. Supports both remote (vector API) and local (ragCore/ directory tree) modes. Use to find reference passages, genre conventions, popular tropes, writing styles, and narrative techniques from successful novels in the knowledge base."
}

func (ragSearch) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "query":{"type":"string","description":"Search query — describe what kind of novel reference you need (e.g. '玄幻修仙逆袭打脸爽点写法', '都市神医治病桥段')"},
  "top_k":{"type":"integer","description":"Number of documents to retrieve (1-20, default 5)","default":5}
},
"required":["query"]
}`)
}

func (ragSearch) ReadOnly() bool { return true }

func (ragSearch) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if !ragCfg.Enabled {
		return "", fmt.Errorf("RAG search is not configured. Use /rag init <path> to set up local knowledge base, or /rag remote to configure a remote service.")
	}

	var params struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("rag_search: invalid arguments: %w", err)
	}
	if params.Query == "" {
		return "", fmt.Errorf("rag_search: query is required")
	}
	if params.TopK <= 0 {
		params.TopK = ragCfg.TopK
		if params.TopK <= 0 {
			params.TopK = 5
		}
	}
	if params.TopK > 20 {
		params.TopK = 20
	}

	if ragCfg.Mode == "local" && ragCfg.LocalPath != "" {
		return localSearch(ctx, params.Query, params.TopK)
	}
	return remoteSearch(ctx, params.Query, params.TopK)
}

// --- remote search (vector API) ---

func remoteSearch(ctx context.Context, query string, topK int) (string, error) {
	if ragCfg.Endpoint == "" {
		return "", fmt.Errorf("RAG remote endpoint is not configured. Set [rag].endpoint in novel-agent.toml.")
	}
	apiKey := os.Getenv(ragCfg.APIKeyEnv)
	if apiKey == "" {
		return "", fmt.Errorf("RAG API key not found: environment variable %s is empty or not set", ragCfg.APIKeyEnv)
	}

	payload := map[string]any{
		"query":      query,
		"top_k":      topK,
		"index_name": ragCfg.IndexName,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("rag_search: marshal request: %w", err)
	}

	endpoint := strings.TrimRight(ragCfg.Endpoint, "/") + "/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("rag_search: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: ragTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("rag_search: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("rag_search: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("rag_search: server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Results []struct {
			Content  string         `json:"content"`
			Score    float64        `json:"score"`
			Metadata map[string]any `json:"metadata"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	if len(result.Results) == 0 {
		return "No relevant novel references found for this query. Try a different query or broader search terms.", nil
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("Found %d relevant novel reference(s) for query: %s\n\n", len(result.Results), query))
	for i, r := range result.Results {
		out.WriteString(fmt.Sprintf("--- Reference %d (relevance: %.2f) ---\n", i+1, r.Score))
		if title, ok := r.Metadata["title"].(string); ok {
			out.WriteString(fmt.Sprintf("Source: %s\n", title))
		}
		out.WriteString(r.Content)
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String()), nil
}

// --- local search (ragCore directory tree) ---
//
// Standard ragCore layout:
//   ragCore/
//     {genre}/                  ← one of: xuanhuan, dushi, guyan, xuanyi, kehuan, tianchong
//       {rank}_{title}/         ← e.g. 1_斗破苍穹
//         {volume}/             ← optional: 第一卷, 第二卷, ...
//           chapter{N}.txt      ← chapter1.txt, chapter2.txt, ...
//
// Variants also accepted:
//   - chapter files directly under the title dir (no volume sub-dir)
//   - 第N章.txt as chapter filename instead of chapter{N}.txt

// genreNames maps genre codes to Chinese labels.
var genreNames = map[string]string{
	"xuanhuan":  "玄幻修仙",
	"dushi":     "都市网文",
	"guyan":     "古言权谋",
	"xuanyi":    "悬疑灵异",
	"kehuan":    "科幻无限",
	"tianchong": "现言甜宠",
}

// chapterFileRe matches chapter files: chapter1.txt / 第1章.txt / chapter{数字}.txt
var chapterFileRe = regexp.MustCompile(`(?i)^(?:chapter\s*(\d+)|第\s*(\d+)\s*章)\.(txt|md)$`)

// searchHit is one matched snippet.
type searchHit struct {
	score   float64
	content string
	source  string // human-readable path: "玄幻修仙 / 1_斗破苍穹 / 第一卷 / chapter1.txt"
}

func localSearch(ctx context.Context, query string, topK int) (string, error) {
	root := ragCfg.LocalPath
	if root == "" {
		return "", fmt.Errorf("local RAG path is empty")
	}

	// Simple keyword-based search: split query into terms, score by term density.
	terms := splitSearchTerms(query)
	if len(terms) == 0 {
		return "No search terms extracted from query.", nil
	}

	var hits []searchHit
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if !chapterFileRe.MatchString(name) {
			return nil
		}

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)

		// Score by term occurrence
		score := scoreText(terms, text)
		if score <= 0 {
			return nil
		}

		// Build a human-readable source path relative to root
		rel, _ := filepath.Rel(root, path)
		source := formatSourcePath(rel)

		// Extract a snippet around the best match
		snippet := extractSnippet(text, terms, 500)

		hits = append(hits, searchHit{
			score:   score,
			content: snippet,
			source:  source,
		})
		return nil
	})

	if walkErr != nil && walkErr != filepath.SkipDir {
		return "", fmt.Errorf("rag_search: walk local path: %w", walkErr)
	}

	if len(hits) == 0 {
		return fmt.Sprintf("No matching chapters found in %s for query: %s", root, query), nil
	}

	// Sort by score descending, take topK
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topK {
		hits = hits[:topK]
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("Found %d relevant chapter(s) for query: %s\n\n", len(hits), query))
	for i, h := range hits {
		out.WriteString(fmt.Sprintf("--- Reference %d (score: %.2f) ---\n", i+1, h.score))
		out.WriteString(fmt.Sprintf("Source: %s\n", h.source))
		out.WriteString(h.content)
		out.WriteString("\n\n")
	}
	return strings.TrimSpace(out.String()), nil
}

// splitSearchTerms splits a query into individual search terms, filtering out
// short or common words.
func splitSearchTerms(query string) []string {
	// For Chinese text, treat each non-whitespace run as a term candidate.
	// Use simple whitespace + punctuation splitting.
	raw := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '，' || r == '。' || r == '、' || r == '；'
	})
	var terms []string
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if len([]rune(t)) >= 2 { // at least 2 Chinese chars
			terms = append(terms, t)
		}
	}
	return terms
}

// scoreText computes a simple density score: sum of per-term occurrence counts
// divided by total text length (to favor shorter, denser chapters).
func scoreText(terms []string, text string) float64 {
	textLen := float64(len([]rune(text)))
	if textLen < 10 {
		return 0
	}
	var totalHits float64
	lowerText := strings.ToLower(text)
	for _, term := range terms {
		count := strings.Count(lowerText, strings.ToLower(term))
		totalHits += float64(count)
	}
	if totalHits == 0 {
		return 0
	}
	// Density score: hits per 1000 chars, plus absolute hit count bonus
	density := totalHits / textLen * 1000
	return density + totalHits*0.5
}

// formatSourcePath converts a relative file path into a human-readable source label.
// E.g. "xuanhuan/1_斗破苍穹/第一卷/chapter3.txt" → "玄幻修仙 / 1_斗破苍穹 / 第一卷 / chapter3.txt"
func formatSourcePath(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, p := range parts {
		if label, ok := genreNames[p]; ok {
			parts[i] = label
		}
	}
	return strings.Join(parts, " / ")
}

// extractSnippet returns a window of text around the first occurrence of any term.
func extractSnippet(text string, terms []string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return strings.TrimSpace(text)
	}

	lowerText := strings.ToLower(text)
	bestPos := -1
	for _, term := range terms {
		idx := strings.Index(lowerText, strings.ToLower(term))
		if idx >= 0 && (bestPos < 0 || idx < bestPos) {
			bestPos = idx
		}
	}

	if bestPos < 0 {
		return strings.TrimSpace(string(runes[:maxRunes])) + "..."
	}

	// Convert byte position to rune position roughly
	runePos := len([]rune(text[:bestPos]))
	start := runePos - maxRunes/3
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}

	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}
	return snippet
}
