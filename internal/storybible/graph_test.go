package storybible

import (
	"encoding/json"
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil {
		t.Fatal("NewGraph returned nil")
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatal("new graph should be empty")
	}
}

func TestAddAndGetNode(t *testing.T) {
	g := NewGraph()
	n := g.AddNode(Node{
		Name: "叶凡",
		Kind: KindCharacter,
		Properties: map[string]interface{}{
			"实力": "筑基期",
			"势力": "青云宗",
		},
	})
	if n.ID == "" {
		t.Fatal("node ID should be auto-generated")
	}

	got := g.GetNode(n.ID)
	if got == nil || got.Name != "叶凡" {
		t.Fatalf("GetNode returned %v", got)
	}
	if got.Properties["实力"] != "筑基期" {
		t.Fatalf("properties mismatch: %v", got.Properties)
	}
}

func TestUpdateNodeProperties(t *testing.T) {
	g := NewGraph()
	n := g.AddNode(Node{
		Name: "叶凡",
		Kind: KindCharacter,
		Properties: map[string]interface{}{
			"实力": "筑基期",
		},
	})

	err := g.UpdateNodeProperties(n.ID, map[string]interface{}{
		"实力": "金丹期",
		"状态": "受伤",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := g.GetNode(n.ID)
	if got.Properties["实力"] != "金丹期" {
		t.Fatalf("实力 should be 金丹期, got %v", got.Properties["实力"])
	}
	if got.Properties["状态"] != "受伤" {
		t.Fatalf("状态 should be 受伤, got %v", got.Properties["状态"])
	}
}

func TestAddAndFindEdge(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})

	e := g.AddEdge(Edge{
		From: charA.ID,
		To:   charB.ID,
		Kind: RelAlly,
	})
	if e.ID == "" {
		t.Fatal("edge ID should be auto-generated")
	}

	found := g.FindEdge(charA.ID, charB.ID, RelAlly)
	if found == nil {
		t.Fatal("FindEdge should return the edge")
	}
	if found.From != charA.ID || found.To != charB.ID {
		t.Fatal("edge endpoints mismatch")
	}

	// Negative test
	if g.FindEdge(charA.ID, charB.ID, RelEnemy) != nil {
		t.Fatal("should not find enemy edge")
	}
}

func TestUpsertEdge(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})

	// First upsert creates
	e1 := g.UpsertEdge(Edge{
		From: charA.ID,
		To:   charB.ID,
		Kind: RelAlly,
		Properties: map[string]string{"intensity": "low"},
	})

	// Second upsert with same from/to/kind updates
	e2 := g.UpsertEdge(Edge{
		From:       charA.ID,
		To:         charB.ID,
		Kind:       RelAlly,
		Properties: map[string]string{"intensity": "high"},
	})

	if e1.ID != e2.ID {
		t.Fatalf("upsert should reuse existing edge ID: %s vs %s", e1.ID, e2.ID)
	}
	if e2.Properties["intensity"] != "high" {
		t.Fatalf("upsert should update properties, got %v", e2.Properties)
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})
	g.AddEdge(Edge{From: charA.ID, To: charB.ID, Kind: RelAlly})

	g.RemoveNode(charA.ID)
	if g.GetNode(charA.ID) != nil {
		t.Fatal("removed node should not be found")
	}
	// Edge should also be removed
	if len(g.AllEdges()) != 0 {
		t.Fatal("edges connecting to removed node should be deleted")
	}
}

func TestSnapshot(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter, Properties: map[string]interface{}{"实力": "金丹期"}})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})
	charC := g.AddNode(Node{Name: "青云宗", Kind: KindFaction})
	charD := g.AddNode(Node{Name: "魔教", Kind: KindFaction})

	g.AddEdge(Edge{From: charA.ID, To: charB.ID, Kind: RelAlly})          // A-B
	g.AddEdge(Edge{From: charA.ID, To: charC.ID, Kind: RelBelongsTo})     // A-C
	g.AddEdge(Edge{From: charC.ID, To: charD.ID, Kind: RelEnemy})         // C-D

	// Snapshot centered on charA at depth 1
	snap := g.SnapshotFor([]string{charA.ID}, 1)

	if len(snap.Nodes) != 3 {
		t.Fatalf("snapshot should have 3 nodes (A + B + C), got %d", len(snap.Nodes))
	}
	if snap.Nodes[charA.ID] == nil {
		t.Fatal("charA should be in snapshot")
	}
	if snap.Nodes[charB.ID] == nil {
		t.Fatal("charB (neighbor of A) should be in snapshot")
	}
	if snap.Nodes[charC.ID] == nil {
		t.Fatal("charC (neighbor of A) should be in snapshot")
	}
	if snap.Nodes[charD.ID] != nil {
		t.Fatal("charD (depth 2) should NOT be in snapshot")
	}
	// Edge A-C and A-B should be in snapshot, but C-D should not (D not included)
	if len(snap.Edges) != 2 {
		t.Fatalf("snapshot should have 2 edges, got %d", len(snap.Edges))
	}
}

func TestSnapshotDepth0(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})
	g.AddEdge(Edge{From: charA.ID, To: charB.ID, Kind: RelAlly})

	snap := g.SnapshotFor([]string{charA.ID}, 0)
	if len(snap.Nodes) != 1 || len(snap.Edges) != 0 {
		t.Fatalf("depth-0 snapshot should only include seed nodes, got nodes=%d edges=%d",
			len(snap.Nodes), len(snap.Edges))
	}
}

func TestApplyBatch(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter, Properties: map[string]interface{}{"实力": "筑基期"}})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})

	batch := UpdateBatch{
		Chapter: 1,
		Instructions: []UpdateInstruction{
			{
				Op:     OpSetProp,
				NodeID: charA.ID,
				Props:  map[string]interface{}{"实力": "金丹期"},
				Reason: "突破",
			},
			{
				Op:     OpAddEdge,
				FromID: charA.ID,
				ToID:   charB.ID,
				RelKind: RelAlly,
				Reason: "初次结盟",
			},
		},
	}

	errs := g.ApplyBatch(batch)
	if len(errs) != 0 {
		t.Fatalf("apply errors: %v", errs)
	}

	// Verify property change
	if g.GetNode(charA.ID).Properties["实力"] != "金丹期" {
		t.Fatal("实力 should be 金丹期")
	}

	// Verify edge
	if g.FindEdge(charA.ID, charB.ID, RelAlly) == nil {
		t.Fatal("edge should exist")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Name: "叶凡", Kind: KindCharacter, Properties: map[string]interface{}{"实力": "筑基期"}})
	g.AddNode(Node{Name: "青云宗", Kind: KindFaction})

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}

	g2 := NewGraph()
	if err := json.Unmarshal(data, g2); err != nil {
		t.Fatal(err)
	}
	if len(g2.Nodes) != 2 {
		t.Fatalf("deserialized graph should have 2 nodes, got %d", len(g2.Nodes))
	}
}

func TestDescribeInstruction(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter, Properties: map[string]interface{}{"实力": "筑基期"}})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})

	inst := UpdateInstruction{
		Op:     OpSetProp,
		NodeID: charA.ID,
		Props:  map[string]interface{}{"实力": "金丹期"},
	}
	desc := DescribeInstruction(inst, g)
	if desc == "" {
		t.Fatal("description should not be empty")
	}
	t.Logf("property change: %s", desc)

	inst2 := UpdateInstruction{
		Op:      OpAddEdge,
		FromID:  charA.ID,
		ToID:    charB.ID,
		RelKind: RelAlly,
		Reason:  "共同御敌",
	}
	desc2 := DescribeInstruction(inst2, g)
	if desc2 == "" {
		t.Fatal("edge description should not be empty")
	}
	t.Logf("edge change: %s", desc2)
}

func TestNodesByKind(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})
	g.AddNode(Node{Name: "青云宗", Kind: KindFaction})

	chars := g.NodesByKind(KindCharacter)
	if len(chars) != 2 {
		t.Fatalf("expected 2 characters, got %d", len(chars))
	}
	factions := g.NodesByKind(KindFaction)
	if len(factions) != 1 {
		t.Fatalf("expected 1 faction, got %d", len(factions))
	}
}

func TestConcurrency(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			g.AddNode(Node{Name: "Node", Kind: KindCharacter})
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = g.AllNodes()
		}
		done <- struct{}{}
	}()

	<-done
	<-done
	// No race detector failures = pass
}

func TestNeighbors(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})
	charC := g.AddNode(Node{Name: "青云宗", Kind: KindFaction})

	g.AddEdge(Edge{From: charA.ID, To: charB.ID, Kind: RelAlly})
	g.AddEdge(Edge{From: charA.ID, To: charC.ID, Kind: RelBelongsTo})

	neighbors := g.Neighbors(charA.ID)
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}
}

func TestUpdateNodeUnknown(t *testing.T) {
	g := NewGraph()
	err := g.UpdateNodeProperties("nonexistent", map[string]interface{}{"x": "y"})
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
}

func TestRemoveEdgesBetween(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})
	charB := g.AddNode(Node{Name: "林婉儿", Kind: KindCharacter})

	g.AddEdge(Edge{From: charA.ID, To: charB.ID, Kind: RelAlly})
	g.RemoveEdgesBetween(charA.ID, charB.ID)
	if len(g.AllEdges()) != 0 {
		t.Fatal("edges should be removed")
	}
}

// Ensure time fields are set on AddNode and AddEdge.
func TestTimestamps(t *testing.T) {
	g := NewGraph()
	n := g.AddNode(Node{Name: "Test", Kind: KindCharacter})
	if n.CreatedAt.IsZero() || n.UpdatedAt.IsZero() {
		t.Fatal("timestamps should be set")
	}
	e := g.AddEdge(Edge{From: n.ID, To: n.ID, Kind: RelCustom})
	if e.CreatedAt.IsZero() {
		t.Fatal("edge CreatedAt should be set")
	}
}

func TestDescribeBatch(t *testing.T) {
	g := NewGraph()
	charA := g.AddNode(Node{Name: "叶凡", Kind: KindCharacter})

	batch := UpdateBatch{
		Chapter: 3,
		Instructions: []UpdateInstruction{
			{Op: OpSetProp, NodeID: charA.ID, Props: map[string]interface{}{"实力": "元婴期"}},
		},
	}
	lines := DescribeBatch(batch, g)
	if len(lines) != 1 {
		t.Fatal("expected 1 description line")
	}
	t.Logf("batch description: %s", lines[0])
}
