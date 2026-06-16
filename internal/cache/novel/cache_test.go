package novelcache

import (
	"strings"
	"testing"
)

func TestAssetCache(t *testing.T) {
	c := NewAssetCache()
	if c.Count() != 0 {
		t.Fatalf("empty cache should have 0 assets, got %d", c.Count())
	}
	if c.Render() != "" {
		t.Fatalf("empty cache should render empty, got %q", c.Render())
	}

	c.Set(Asset{Type: AssetWorldbuilding, Content: "修真界，人间界"})
	if c.Count() != 1 {
		t.Fatalf("expected 1 asset, got %d", c.Count())
	}
	rendered := c.Render()
	if !strings.Contains(rendered, "修真界") {
		t.Fatalf("render should contain asset content: %s", rendered)
	}
	if !strings.Contains(rendered, "<writing-assets>") {
		t.Fatalf("render should wrap in <writing-assets>: %s", rendered)
	}

	// Hash should be stable.
	h1 := c.Hash()
	h2 := c.Hash()
	if h1 != h2 {
		t.Fatalf("hash should be stable: %s != %s", h1, h2)
	}

	c.Set(Asset{Type: AssetCharacters, Content: "叶凡：主角"})
	if c.Count() != 2 {
		t.Fatalf("expected 2 assets, got %d", c.Count())
	}
	h3 := c.Hash()
	if h3 == h1 {
		t.Fatalf("hash should change after adding asset: %s == %s", h3, h1)
	}

	c.Remove(AssetWorldbuilding)
	if c.Count() != 1 {
		t.Fatalf("expected 1 asset after removal, got %d", c.Count())
	}
}

func TestFragmentCache(t *testing.T) {
	c := NewFragmentCache()
	if c.Count() != 0 {
		t.Fatalf("empty cache should have 0 fragments, got %d", c.Count())
	}

	c.Add(Fragment{
		ID:        "face-slapping-1",
		Genre:     "xuanhuan",
		TropeType: "打脸",
		Content:   "主角在众人面前碾压对手",
		Score:     1.0,
		Tags:      []string{"高潮", "逆袭"},
	})
	c.Add(Fragment{
		ID:        "face-slapping-2",
		Genre:     "dushi",
		TropeType: "打脸",
		Content:   "主角在宴会上揭穿对手",
		Score:     0.8,
		Tags:      []string{"高潮", "社交"},
	})

	if c.Count() != 2 {
		t.Fatalf("expected 2 fragments, got %d", c.Count())
	}

	items := c.GetByTrope("xuanhuan", "打脸")
	if len(items) != 1 {
		t.Fatalf("expected 1 xuanhuan fragment, got %d", len(items))
	}

	items = c.GetByTags("", []string{"高潮"})
	if len(items) != 2 {
		t.Fatalf("expected 2 fragments matching tag, got %d", len(items))
	}
}

func TestSummaryCache(t *testing.T) {
	c := NewSummaryCache(3)
	if c.Count() != 0 {
		t.Fatalf("empty cache should have 0 summaries, got %d", c.Count())
	}

	c.Push(ChapterSummary{ChapterNum: 1, Title: "第1章", Summary: "主角觉醒金手指", Tokens: 10})
	c.Push(ChapterSummary{ChapterNum: 2, Title: "第2章", Summary: "主角进入宗门", Tokens: 8})
	c.Push(ChapterSummary{ChapterNum: 3, Title: "第3章", Summary: "主角通过试炼", Tokens: 12})

	if c.Count() != 3 {
		t.Fatalf("expected 3 summaries, got %d", c.Count())
	}

	latest := c.Latest()
	if latest == nil || latest.ChapterNum != 3 {
		t.Fatalf("latest should be chapter 3, got %+v", latest)
	}

	c.Push(ChapterSummary{ChapterNum: 4, Title: "第4章", Summary: "主角突破", Tokens: 6})
	if c.Count() != 3 {
		t.Fatalf("window cap should keep at most 3, got %d", c.Count())
	}

	rendered := c.RenderAll()
	if !strings.Contains(rendered, "ch.2") {
		t.Fatalf("render should contain chapter 2: %s", rendered)
	}
	if strings.Contains(rendered, "ch.1") {
		t.Fatalf("render should NOT contain chapter 1 (evicted): %s", rendered)
	}

	short := c.Render(2)
	if strings.Count(short, "ch.") > 2 {
		t.Fatalf("render(2) should contain at most 2 chapters: %s", short)
	}

	tokens := c.TotalTokens()
	if tokens <= 0 {
		t.Fatalf("total tokens should be positive, got %d", tokens)
	}

	c.Clear()
	if c.Count() != 0 {
		t.Fatalf("after clear should be empty, got %d", c.Count())
	}
}

// --- Benchmarks ---

func BenchmarkAssetCacheSet(b *testing.B) {
	c := NewAssetCache()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(Asset{
			Type:    AssetWorldbuilding,
			Content: "修真界，人间界。境界体系：炼气、筑基、金丹、元婴、化神",
		})
	}
}

func BenchmarkAssetCacheRender(b *testing.B) {
	c := NewAssetCache()
	c.Set(Asset{Type: AssetWorldbuilding, Content: "修真界"})
	c.Set(Asset{Type: AssetCharacters, Content: "叶凡：主角"})
	c.Set(Asset{Type: AssetOutline, Content: "五卷大纲"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Render()
	}
}

func BenchmarkFragmentCacheAdd(b *testing.B) {
	c := NewFragmentCache()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Add(Fragment{
			ID:        "frag",
			Genre:     "xuanhuan",
			TropeType: "打脸",
			Content:   "打脸桥段模板内容",
			Score:     1.0,
		})
	}
}

func BenchmarkFragmentCacheGet(b *testing.B) {
	c := NewFragmentCache()
	for i := 0; i < 100; i++ {
		c.Add(Fragment{
			ID:        "f",
			Genre:     "xuanhuan",
			TropeType: "打脸",
			Content:   "打脸模板",
			Score:     float64(i) / 100.0,
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.GetByTrope("xuanhuan", "打脸")
	}
}

func BenchmarkSummaryCachePush(b *testing.B) {
	c := NewSummaryCache(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Push(ChapterSummary{
			ChapterNum: i,
			Summary:    "主角突破金丹境界",
			Tokens:     10,
		})
	}
}

func BenchmarkSummaryCacheRender(b *testing.B) {
	c := NewSummaryCache(20)
	for i := 0; i < 20; i++ {
		c.Push(ChapterSummary{ChapterNum: i, Summary: "主角突破金丹", Tokens: 10})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RenderAll()
	}
}
