package domain

import (
	"fmt"
	"sort"
)

type PreflightBlocker struct {
	Code      string `json:"code"`
	Scope     string `json:"scope"`
	NodeID    string `json:"node_id,omitempty"`
	Edge      string `json:"edge,omitempty"`
	ListIndex int    `json:"list_index,omitempty"`
	Message   string `json:"message"`
}

type BaselineStats struct {
	NodeCount        int `json:"node_count"`
	EdgeCount        int `json:"edge_count"`
	EntranceCount    int `json:"entrance_count"`
	DestinationCount int `json:"destination_count"`
	BlockerCount     int `json:"blocker_count"`
}

type BaselinePreflight struct {
	Summary  string             `json:"summary"`
	Stats    BaselineStats      `json:"stats"`
	Blockers []PreflightBlocker `json:"blockers"`
	Graph    SurveyGraph        `json:"graph"`
}

func PreflightBaseline(input SurveyGraph) BaselinePreflight {
	g := NormalizeGraph(input)
	g.BaselineDigest = ""
	blockers := make([]PreflightBlocker, 0)
	add := func(code, scope, node, edge string, index int, message string) {
		blockers = append(blockers, PreflightBlocker{Code: code, Scope: scope, NodeID: node, Edge: edge, ListIndex: index, Message: message})
	}
	nodes := map[string]Node{}
	for i, n := range g.Nodes {
		if n.ID == "" {
			add("EMPTY_NODE_ID", "node", "", "", i, "节点标识不能为空")
			continue
		}
		if _, ok := nodes[n.ID]; ok {
			add("DUPLICATE_NODE_ID", "node", n.ID, "", i, fmt.Sprintf("节点 %s 重复", n.ID))
			continue
		}
		nodes[n.ID] = n
	}
	checkIdentity := func(ids []string, wanted, scope string) {
		seen := map[string]bool{}
		for i, id := range ids {
			if seen[id] {
				add("DUPLICATE_"+scope+"_ID", "identity", id, "", i, fmt.Sprintf("%s标识 %s 重复", identityName(scope), id))
			}
			seen[id] = true
			n, ok := nodes[id]
			if !ok {
				add("UNKNOWN_"+scope+"_NODE", "identity", id, "", i, fmt.Sprintf("%s %s 指向不存在节点", identityName(scope), id))
				continue
			}
			if n.Kind != wanted {
				add(scope+"_KIND_MISMATCH", "identity", id, "", i, fmt.Sprintf("节点 %s 类型应为 %s", id, wanted))
			}
		}
	}
	checkIdentity(g.EntranceIDs, "entrance", "ENTRANCE")
	checkIdentity(g.DestinationIDs, "destination", "DESTINATION")
	edges := map[string]bool{}
	for i, e := range g.Edges {
		key := e.From + "->" + e.To
		if _, ok := nodes[e.From]; !ok {
			add("UNKNOWN_EDGE_FROM", "edge", e.From, key, i, fmt.Sprintf("连接 %s 的起点不存在", key))
		}
		if _, ok := nodes[e.To]; !ok {
			add("UNKNOWN_EDGE_TO", "edge", e.To, key, i, fmt.Sprintf("连接 %s 的终点不存在", key))
		}
		if e.From == e.To {
			add("SELF_EDGE", "edge", e.From, key, i, fmt.Sprintf("连接 %s 不得自连接", key))
		}
		if edges[key] {
			add("DUPLICATE_EDGE", "edge", e.From, key, i, fmt.Sprintf("连接 %s 重复", key))
		}
		edges[key] = true
		if flag, ok := g.AccessibleEdgeFlags[key]; ok && flag != e.Accessible {
			add("ACCESSIBLE_FLAG_MISMATCH", "edge", e.From, key, i, fmt.Sprintf("连接 %s 的无障碍标记不一致", key))
		}
	}
	for key := range g.AccessibleEdgeFlags {
		if !edges[key] {
			add("ORPHAN_ACCESSIBLE_FLAG", "edge", "", key, 0, fmt.Sprintf("无障碍约束 %s 没有对应连接", key))
		}
	}
	reachable := Reachable(g)
	for _, id := range uniqueSorted(g.DestinationIDs) {
		if _, ok := nodes[id]; ok && !reachable[id] {
			add("UNREACHABLE_DESTINATION", "identity", id, "", 0, fmt.Sprintf("目的地 %s 无法从任何入口到达", id))
		}
	}
	sort.SliceStable(blockers, func(i, j int) bool {
		a, b := blockers[i], blockers[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.Edge != b.Edge {
			return a.Edge < b.Edge
		}
		return a.ListIndex < b.ListIndex
	})
	stats := BaselineStats{len(g.Nodes), len(g.Edges), len(g.EntranceIDs), len(g.DestinationIDs), len(blockers)}
	summaryInput := struct {
		Graph    SurveyGraph        `json:"graph"`
		Stats    BaselineStats      `json:"stats"`
		Blockers []PreflightBlocker `json:"blockers"`
	}{g, stats, blockers}
	return BaselinePreflight{Summary: Digest(summaryInput), Stats: stats, Blockers: blockers, Graph: g}
}

func identityName(scope string) string {
	if scope == "ENTRANCE" {
		return "入口"
	}
	return "目的地"
}
func uniqueSorted(ids []string) []string {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
