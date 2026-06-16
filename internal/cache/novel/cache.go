// Package novelcache provides a three-level writing asset cache for novel
// creation: global assets (permanent), semantic fragments (reusable tropes),
// and plot summaries (rolling window). Together they replace the repeated
// injection of full context into every prompt turn — assets load once into
// the cache-stable system prefix, fragments are recalled by semantic tag,
// and summaries keep the model up to date without re-feeding entire chapters.
//
// Key design principles:
//   - Level 1 (asset) content joins the system prompt prefix, so it is
//     cache-stable and costs 0 extra tokens per turn after the first.
//   - Level 2 (fragment) is read-only Go logic: the Agent calls it before
//     building a turn, and the result is a compact reference, not raw text.
//   - Level 3 (summary) updates are cheap and automatic; only the latest N
//     summaries are kept in the active session to bound the token budget.
package novelcache

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- Level 1: Global Asset Cache ---

// AssetType categorises a cached writing asset.
type AssetType string

const (
	AssetWorldbuilding AssetType = "worldbuilding" // 世界观设定
	AssetCharacters   AssetType = "characters"    // 人设卡
	AssetPower        AssetType = "power"         // 金手指/力量体系
	AssetOutline      AssetType = "outline"       // 全书粗纲
	AssetHooks        AssetType = "hooks"         // 伏笔清单
	AssetStyle        AssetType = "style"         // 风格配置
)

// Asset is a single cacheable writing asset.
type Asset struct {
	Type      AssetType   // asset category
	Version   int         // bumped on each update; used in the prefix hash
	Content   string      // the full text to inject
	UpdatedAt time.Time   // last modification time
}

// AssetCache holds the permanent writing assets for one project. It is
// populated at boot from the project's .novelAgent/ directory and its
// Render method produces the text fragment that joins the system prompt.
type AssetCache struct {
	mu     sync.RWMutex
	assets map[AssetType]*Asset
	hash   string // combined hash of all asset contents + versions
}

// NewAssetCache creates an empty asset cache.
func NewAssetCache() *AssetCache {
	return &AssetCache{
		assets: make(map[AssetType]*Asset),
	}
}

// Set stores or updates an asset. It recomputes the combined hash so the
// system-prompt fingerprint stays accurate.
func (c *AssetCache) Set(a Asset) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a.Version++
	if existing, ok := c.assets[a.Type]; ok {
		a.Version = existing.Version + 1
	}
	a.UpdatedAt = time.Now()
	c.assets[a.Type] = &a
	c.rehash()
}

// Get retrieves an asset by type. Returns nil when absent.
func (c *AssetCache) Get(t AssetType) *Asset {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.assets[t]
}

// Remove deletes an asset by type and recomputes the hash.
func (c *AssetCache) Remove(t AssetType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.assets, t)
	c.rehash()
}

// Hash returns the combined content hash — stable as long as no asset
// changes, so it can join the prefix fingerprint in PrefixShape.
func (c *AssetCache) Hash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hash
}

// Render produces the system-prompt injection text: a compact, tagged block
// listing every cached asset. Returns empty when the cache is empty (so the
// prefix stays unchanged).
func (c *AssetCache) Render() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.assets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n<writing-assets>\n")
	// Emit in deterministic order so the hash is stable.
	for _, t := range sortedAssetTypes(c.assets) {
		a := c.assets[t]
		b.WriteString(fmt.Sprintf("<asset type=%q version=%d>\n", string(a.Type), a.Version))
		b.WriteString(a.Content)
		b.WriteString("\n</asset>\n")
	}
	b.WriteString("</writing-assets>\n")
	return b.String()
}

// Count returns the number of cached assets.
func (c *AssetCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.assets)
}

func (c *AssetCache) rehash() {
	// Simple content digest for fingerprinting.
	h := 0
	for _, t := range sortedAssetTypes(c.assets) {
		a := c.assets[t]
		h = h*31 + len(a.Content) + a.Version*7
	}
	c.hash = fmt.Sprintf("v%x", h)
}

// sortedAssetTypes returns the asset types in sorted order for stable output.
func sortedAssetTypes(m map[AssetType]*Asset) []AssetType {
	out := make([]AssetType, 0, len(m))
	for t := range m {
		out = append(out, t)
	}
	// Simple sort by type name.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i] > out[j] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// --- Level 2: Semantic Fragment Cache ---

// Fragment is a reusable writing fragment (trope, scene template, style
// snippet) tagged for semantic recall.
type Fragment struct {
	ID        string   // unique identifier
	Genre     string   // 玄幻/都市/古言/悬疑/科幻/甜宠
	TropeType string   // 打脸/升级/夺宝/退婚/相认/救场/告白/...
	Content   string   // the reusable text
	Score     float64  // usage frequency / relevance score
	Tags      []string // additional semantic tags for matching
}

// FragmentCache stores reusable writing fragments keyed by
// {genre}:{trope_type}. It is read-heavy, write-light: fragments are
// installed by skills (e.g. /novel-trope-reference caches its output) and
// recalled by the generation flow.
type FragmentCache struct {
	mu    sync.RWMutex
	items map[string][]*Fragment // key = "genre:trope_type"
}

// NewFragmentCache creates an empty fragment cache.
func NewFragmentCache() *FragmentCache {
	return &FragmentCache{items: make(map[string][]*Fragment)}
}

// Add inserts or replaces a fragment.
func (c *FragmentCache) Add(f Fragment) {
	key := f.Genre + ":" + f.TropeType
	c.mu.Lock()
	defer c.mu.Unlock()
	existing := c.items[key]
	for i, ef := range existing {
		if ef.ID == f.ID {
			existing[i] = &f
			return
		}
	}
	c.items[key] = append(existing, &f)
}

// GetByTrope returns all fragments for a given genre and trope type, ordered
// by score descending.
func (c *FragmentCache) GetByTrope(genre, tropeType string) []*Fragment {
	key := genre + ":" + tropeType
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := c.items[key]
	if len(items) == 0 {
		return nil
	}
	out := make([]*Fragment, len(items))
	copy(out, items)
	// Sort by score descending.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[i].Score {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// GetByTags returns fragments matching at least one of the given tags,
// ordered by score descending.
func (c *FragmentCache) GetByTags(genre string, tags []string) []*Fragment {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	var out []*Fragment
	for key, items := range c.items {
		if !strings.HasPrefix(key, genre+":") && genre != "" {
			continue
		}
		for _, f := range items {
			for _, t := range f.Tags {
				if tagSet[t] {
					out = append(out, f)
					break
				}
			}
		}
	}
	// Sort by score.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Score > out[i].Score {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Count returns the total number of fragments.
func (c *FragmentCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n := 0
	for _, items := range c.items {
		n += len(items)
	}
	return n
}

// --- Level 3: Plot Summary Cache (rolling window) ---

// ChapterSummary holds the compact summary of one chapter.
type ChapterSummary struct {
	ChapterNum int    // chapter number (1-based)
	Title      string // chapter title or "第N章"
	Summary    string // one-paragraph plot summary
	Tokens     int    // estimated token count
}

// SummaryCache maintains a sliding window of the most recent N chapter
// summaries. The Agent renders these into a compact context block before
// each writing turn, so the model knows the recent plot without consuming
// the full chapter text.
type SummaryCache struct {
	mu     sync.Mutex
	window int                 // max summaries to keep
	items  []ChapterSummary    // newest first (index 0 = latest)
}

// NewSummaryCache creates a summary cache with the given window size.
func NewSummaryCache(window int) *SummaryCache {
	if window < 1 {
		window = 20 // sensible default
	}
	return &SummaryCache{window: window}
}

// Push adds a new summary. If the window is full, the oldest is dropped.
func (c *SummaryCache) Push(s ChapterSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append([]ChapterSummary{s}, c.items...)
	if len(c.items) > c.window {
		c.items = c.items[:c.window]
	}
}

// Get returns the most recent N summaries (or fewer if not enough exist).
func (c *SummaryCache) Get(n int) []ChapterSummary {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n <= 0 || n > len(c.items) {
		n = len(c.items)
	}
	out := make([]ChapterSummary, n)
	copy(out, c.items[:n])
	return out
}

// Latest returns the single most recent summary, or nil when empty.
func (c *SummaryCache) Latest() *ChapterSummary {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return nil
	}
	return &c.items[0]
}

// Render produces a compact block of recent summaries for context injection.
// Returns empty when there are no summaries. Caller MUST NOT hold c.mu.
func (c *SummaryCache) Render(n int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.renderLocked(n)
}

// renderLocked is the inner implementation of Render; caller must hold c.mu.
func (c *SummaryCache) renderLocked(n int) string {
	if n <= 0 || n > len(c.items) {
		n = len(c.items)
	}
	if n == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<recent-plot window=%d>\n", n))
	// Render oldest first so the model reads chronologically.
	for i := n - 1; i >= 0; i-- {
		s := c.items[i]
		b.WriteString(fmt.Sprintf("  ch.%d", s.ChapterNum))
		if s.Title != "" {
			b.WriteString(" " + s.Title)
		}
		b.WriteString(": " + s.Summary + "\n")
	}
	b.WriteString("</recent-plot>\n")
	return b.String()
}

// RenderAll renders every summary in the window.
func (c *SummaryCache) RenderAll() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.renderLocked(len(c.items))
}

// Clear empties the cache.
func (c *SummaryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
}

// Count returns the number of cached summaries.
func (c *SummaryCache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// TotalTokens returns the estimated total tokens across all summaries.
func (c *SummaryCache) TotalTokens() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, s := range c.items {
		n += s.Tokens
	}
	return n
}
