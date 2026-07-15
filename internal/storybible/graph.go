// Package storybible provides the Story Bible Graph — a structured, persistent
// world-knowledge store that tracks characters, factions, locations, items, and
// events with dynamic relationship edges. It is the source of truth for world
// state and replaces ad-hoc markdown files with a queryable, updatable graph.
//
// Key design invariants:
//   - Every mutation produces an UpdateInstruction that can be replayed / audited.
//   - Snapshots pull only the subgraph relevant to a scene, bounded by depth.
//   - Character Agents and the Timeline Agent consume snapshots — never the full graph.
//   - The graph is serialisable to JSON for lightweight persistence; a future
//     migration path to Neo4j is kept open by the abstract node/edge model.
package storybible

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─── Node Types ────────────────────────────────────────────────────────────

// NodeKind classifies a graph node.
type NodeKind string

const (
	KindCharacter NodeKind = "character" // 角色
	KindFaction   NodeKind = "faction"   // 势力/组织
	KindLocation  NodeKind = "location"  // 地点
	KindItem      NodeKind = "item"      // 物品/功法/道具
	KindEvent     NodeKind = "event"     // 事件
)

// Node is a vertex in the Story Bible Graph. Every node has a stable ID,
// a human-readable name, a kind, and a set of typed properties.
type Node struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Kind       NodeKind               `json:"kind"`
	Properties map[string]interface{} `json:"properties"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// ─── Edge Types ────────────────────────────────────────────────────────────

// RelKind classifies a directed relationship edge.
type RelKind string

const (
	RelAlly      RelKind = "ally"       // 盟友
	RelEnemy     RelKind = "enemy"      // 敌对
	RelSubord    RelKind = "subordinate" // 从属
	RelNeutral   RelKind = "neutral"    // 中立
	RelOwner     RelKind = "owner"      // 持有者 → 物品
	RelLocatedAt RelKind = "located_at" // 角色 → 地点
	RelBelongsTo RelKind = "belongs_to" // 角色 → 势力
	RelMentor    RelKind = "mentor"     // 师徒
	RelFamily    RelKind = "family"     // 血缘/亲属
	RelLover     RelKind = "lover"      // 恋人
	RelRival     RelKind = "rival"      // 竞争对手
	RelBetrayer  RelKind = "betrayer"   // 背叛者
	RelCustom    RelKind = "custom"     // 自定义关系
)

// Edge is a directed, typed relationship between two nodes.
type Edge struct {
	ID         string            `json:"id"`
	From       string            `json:"from"` // source node ID
	To         string            `json:"to"`   // target node ID
	Kind       RelKind           `json:"kind"`
	Properties map[string]string `json:"properties"` // e.g. reason, since_chapter, intensity
	CreatedAt  time.Time         `json:"created_at"`
}

// ─── Graph ─────────────────────────────────────────────────────────────────

// Graph is the full Story Bible Graph. It is concurrency-safe and supports
// incremental updates via UpdateInstruction batches.
type Graph struct {
	mu     sync.RWMutex
	Nodes  map[string]*Node `json:"nodes"`  // id → node
	Edges  map[string]*Edge `json:"edges"`  // id → edge
	nextID int64
}

// NewGraph creates an empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
		Edges: make(map[string]*Edge),
	}
}

// ─── CRUD ──────────────────────────────────────────────────────────────────

// AddNode inserts a node. The ID is auto-generated if empty.
func (g *Graph) AddNode(n Node) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nextID++
	if n.ID == "" {
		n.ID = fmt.Sprintf("%s_%d", n.Kind, g.nextID)
	}
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	g.Nodes[n.ID] = &n
	return g.Nodes[n.ID]
}

// GetNode returns a node by ID, or nil.
func (g *Graph) GetNode(id string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Nodes[id]
}

// UpdateNodeProperties merges new properties into an existing node.
func (g *Graph) UpdateNodeProperties(id string, props map[string]interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	n, ok := g.Nodes[id]
	if !ok {
		return fmt.Errorf("storybible: node %q not found", id)
	}
	if n.Properties == nil {
		n.Properties = make(map[string]interface{})
	}
	for k, v := range props {
		n.Properties[k] = v
	}
	n.UpdatedAt = time.Now()
	return nil
}

// RemoveNode deletes a node and all edges connected to it.
func (g *Graph) RemoveNode(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.Nodes, id)
	for eid, e := range g.Edges {
		if e.From == id || e.To == id {
			delete(g.Edges, eid)
		}
	}
}

// AddEdge inserts a directed edge. The ID is auto-generated.
func (g *Graph) AddEdge(e Edge) *Edge {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nextID++
	if e.ID == "" {
		e.ID = fmt.Sprintf("edge_%d", g.nextID)
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	g.Edges[e.ID] = &e
	return g.Edges[e.ID]
}

// GetEdge returns an edge by ID, or nil.
func (g *Graph) GetEdge(id string) *Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Edges[id]
}

// FindEdge finds the first edge between two nodes with a given kind.
func (g *Graph) FindEdge(from, to string, kind RelKind) *Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, e := range g.Edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return e
		}
	}
	return nil
}

// UpsertEdge replaces an existing edge (same from/to/kind) or inserts a new one.
func (g *Graph) UpsertEdge(e Edge) *Edge {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check for existing edge of same from/to/kind.
	for _, existing := range g.Edges {
		if existing.From == e.From && existing.To == e.To && existing.Kind == e.Kind {
			existing.Properties = e.Properties
			return existing
		}
	}

	g.nextID++
	if e.ID == "" {
		e.ID = fmt.Sprintf("edge_%d", g.nextID)
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	g.Edges[e.ID] = &e
	return g.Edges[e.ID]
}

// RemoveEdge deletes an edge by ID.
func (g *Graph) RemoveEdge(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.Edges, id)
}

// RemoveEdgesBetween deletes all edges between two nodes.
func (g *Graph) RemoveEdgesBetween(a, b string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for eid, e := range g.Edges {
		if (e.From == a && e.To == b) || (e.From == b && e.To == a) {
			delete(g.Edges, eid)
		}
	}
}

// ─── Queries ───────────────────────────────────────────────────────────────

// NodesByKind returns all nodes of a given kind.
func (g *Graph) NodesByKind(kind NodeKind) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Node
	for _, n := range g.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AllNodes returns all nodes.
func (g *Graph) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Node
	for _, n := range g.Nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AllEdges returns all edges.
func (g *Graph) AllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Edge
	for _, e := range g.Edges {
		out = append(out, e)
	}
	return out
}

// Neighbors returns all nodes directly connected to the given node, up to depth 1.
// Result is a map from neighbor node ID to the edge connecting them.
func (g *Graph) Neighbors(nodeID string) map[string][]*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string][]*Edge)
	for _, e := range g.Edges {
		if e.From == nodeID {
			out[e.To] = append(out[e.To], e)
		}
		if e.To == nodeID {
			out[e.From] = append(out[e.From], e)
		}
	}
	return out
}

// ─── Snapshots ─────────────────────────────────────────────────────────────

// Snapshot is a bounded subgraph view. It includes the seed nodes, their
// direct neighbors, and the edges among them — nothing else. This keeps the
// context window small while preserving relevant relationship data.
type Snapshot struct {
	SeedNodeIDs []string          `json:"seed_node_ids"` // the nodes that triggered this snapshot
	Nodes       map[string]*Node  `json:"nodes"`
	Edges       map[string]*Edge  `json:"edges"`
	GeneratedAt time.Time         `json:"generated_at"`
}

// SnapshotFor returns a subgraph snapshot centered on the given seed nodes,
// expanding to depth neighbours (currently only depth=0 and depth=1).
// Depth 0: only the seed nodes themselves.
// Depth 1: seed nodes + their direct neighbors + edges among them.
func (g *Graph) SnapshotFor(seedIDs []string, depth int) *Snapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	s := &Snapshot{
		SeedNodeIDs: seedIDs,
		Nodes:       make(map[string]*Node),
		Edges:       make(map[string]*Edge),
		GeneratedAt: time.Now(),
	}

	// Collect seed nodes and build a set of all included node IDs.
	included := make(map[string]bool)
	for _, id := range seedIDs {
		if n, ok := g.Nodes[id]; ok {
			s.Nodes[id] = n
			included[id] = true
		}
	}

	if depth <= 0 {
		return s
	}

	// Expand one hop.
	for eid, e := range g.Edges {
		fromIn := included[e.From]
		toIn := included[e.To]
		if fromIn || toIn {
			s.Edges[eid] = e
		}
		if fromIn && !toIn {
			if n, ok := g.Nodes[e.To]; ok {
				s.Nodes[e.To] = n
				included[e.To] = true
			}
		}
		if toIn && !fromIn {
			if n, ok := g.Nodes[e.From]; ok {
				s.Nodes[e.From] = n
				included[e.From] = true
			}
		}
	}

	return s
}

// SnapshotForCharacters is a convenience method that builds a snapshot
// centered on a list of character node IDs. Returns nodes + direct neighbors.
func (g *Graph) SnapshotForCharacters(charIDs []string) *Snapshot {
	return g.SnapshotFor(charIDs, 1)
}

// ─── Update Instructions ───────────────────────────────────────────────────

// UpdateOp describes a single atomic change to the graph.
type UpdateOp string

const (
	OpSetProp    UpdateOp = "set_prop"    // 修改节点属性
	OpAddEdge    UpdateOp = "add_edge"    // 添加关系边
	OpRemoveEdge UpdateOp = "remove_edge" // 删除关系边
	OpUpsertEdge UpdateOp = "upsert_edge" // 添加或更新关系边
)

// UpdateInstruction is a single change to the graph, typically produced by
// the Timeline Synthesis Agent after completing a chapter.
type UpdateInstruction struct {
	Op       UpdateOp              `json:"op"`
	NodeID   string                `json:"node_id,omitempty"`   // target node (for set_prop)
	FromID   string                `json:"from_id,omitempty"`   // source node (for add_edge / upsert_edge)
	ToID     string                `json:"to_id,omitempty"`     // target node (for add_edge / upsert_edge)
	RelKind  RelKind               `json:"rel_kind,omitempty"`  // relationship kind (for add_edge / upsert_edge)
	EdgeID   string                `json:"edge_id,omitempty"`   // edge to remove (for remove_edge)
	Props    map[string]interface{} `json:"props,omitempty"`    // properties to set (for set_prop) or edge properties (for add_edge)
	Reason   string                `json:"reason,omitempty"`    // human-readable reason for the change
	Chapter  int                   `json:"chapter,omitempty"`   // which chapter triggered this change
}

// UpdateBatch is a set of UpdateInstructions to be applied atomically.
// The Timeline Agent produces one batch per chapter.
type UpdateBatch struct {
	Chapter      int                  `json:"chapter"`
	Instructions []UpdateInstruction  `json:"instructions"`
	AppliedAt    time.Time            `json:"applied_at"`
}

// ApplyBatch applies all instructions in a batch to the graph.
// Returns any errors encountered (non-fatal — processing continues).
func (g *Graph) ApplyBatch(batch UpdateBatch) []error {
	var errs []error
	for _, inst := range batch.Instructions {
		if err := g.applyInstruction(inst); err != nil {
			errs = append(errs, err)
		}
	}
	batch.AppliedAt = time.Now()
	return errs
}

func (g *Graph) applyInstruction(inst UpdateInstruction) error {
	switch inst.Op {
	case OpSetProp:
		if inst.NodeID == "" {
			return fmt.Errorf("storybible: set_prop requires node_id")
		}
		return g.UpdateNodeProperties(inst.NodeID, inst.Props)

	case OpAddEdge:
		if inst.FromID == "" || inst.ToID == "" {
			return fmt.Errorf("storybible: add_edge requires from_id and to_id")
		}
		e := Edge{
			From: inst.FromID,
			To:   inst.ToID,
			Kind: inst.RelKind,
		}
		if inst.Props != nil {
			e.Properties = make(map[string]string)
			for k, v := range inst.Props {
				e.Properties[k] = fmt.Sprint(v)
			}
		}
		g.AddEdge(e)

	case OpUpsertEdge:
		if inst.FromID == "" || inst.ToID == "" {
			return fmt.Errorf("storybible: upsert_edge requires from_id and to_id")
		}
		e := Edge{
			From: inst.FromID,
			To:   inst.ToID,
			Kind: inst.RelKind,
		}
		if inst.Props != nil {
			e.Properties = make(map[string]string)
			for k, v := range inst.Props {
				e.Properties[k] = fmt.Sprint(v)
			}
		}
		g.UpsertEdge(e)

	case OpRemoveEdge:
		if inst.EdgeID != "" {
			g.RemoveEdge(inst.EdgeID)
		} else if inst.FromID != "" && inst.ToID != "" {
			g.RemoveEdgesBetween(inst.FromID, inst.ToID)
		} else {
			return fmt.Errorf("storybible: remove_edge requires edge_id or (from_id, to_id)")
		}

	default:
		return fmt.Errorf("storybible: unknown op %q", inst.Op)
	}

	return nil
}

// ─── Human-readable description ────────────────────────────────────────────

// DescribeInstruction returns a human-readable string for an update instruction,
// matching the format shown in the design doc (e.g. "角色A.当前实力: 筑基期 → 金丹期").
func DescribeInstruction(inst UpdateInstruction, g *Graph) string {
	switch inst.Op {
	case OpSetProp:
		nodeName := inst.NodeID
		if n := g.GetNode(inst.NodeID); n != nil {
			nodeName = n.Name
		}
		var parts []string
		for k, v := range inst.Props {
			old := ""
			if n := g.GetNode(inst.NodeID); n != nil {
				if ov, ok := n.Properties[k]; ok {
					old = fmt.Sprint(ov)
				}
			}
			if old != "" {
				parts = append(parts, fmt.Sprintf("%s.%s: %s → %s", nodeName, k, old, v))
			} else {
				parts = append(parts, fmt.Sprintf("%s.%s: ∅ → %s", nodeName, k, v))
			}
		}
		return strings.Join(parts, "; ")

	case OpAddEdge, OpUpsertEdge:
		fromName := inst.FromID
		toName := inst.ToID
		if n := g.GetNode(inst.FromID); n != nil {
			fromName = n.Name
		}
		if n := g.GetNode(inst.ToID); n != nil {
			toName = n.Name
		}
		desc := fmt.Sprintf("%s —%s→ %s", fromName, inst.RelKind, toName)
		if inst.Reason != "" {
			desc += fmt.Sprintf("（原因：%s）", inst.Reason)
		}
		return desc

	default:
		return fmt.Sprintf("%s on %s", inst.Op, inst.NodeID)
	}
}

// DescribeBatch returns a human-readable change log for an entire batch.
func DescribeBatch(batch UpdateBatch, g *Graph) []string {
	var lines []string
	for _, inst := range batch.Instructions {
		lines = append(lines, DescribeInstruction(inst, g))
	}
	return lines
}
