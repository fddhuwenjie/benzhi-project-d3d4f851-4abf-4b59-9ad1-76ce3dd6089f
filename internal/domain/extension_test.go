package domain

import "testing"

func TestBaselinePreflightDeterministicAndLocated(t *testing.T) {
	valid := testGraph()
	reordered := valid
	reordered.Nodes = []Node{valid.Nodes[2], valid.Nodes[0], valid.Nodes[1]}
	reordered.Edges = []Edge{valid.Edges[1], valid.Edges[0]}
	first, second := PreflightBaseline(valid), PreflightBaseline(reordered)
	if len(first.Blockers) != 0 || first.Summary != second.Summary {
		t.Fatalf("语义相同的基线预检应稳定: %+v %+v", first, second)
	}
	invalid := valid
	invalid.EntranceIDs = []string{"E", "E"}
	invalid.Edges = append(invalid.Edges, Edge{From: "missing", To: "D"})
	preflight := PreflightBaseline(invalid)
	codes := map[string]bool{}
	for _, blocker := range preflight.Blockers {
		codes[blocker.Code] = true
	}
	if !codes["DUPLICATE_ENTRANCE_ID"] || !codes["UNKNOWN_EDGE_FROM"] {
		t.Fatalf("阻断项没有定位重复身份和未知节点: %+v", preflight.Blockers)
	}
}

func TestSignPreflightNormalizesDiffAndRejectsBatch(t *testing.T) {
	graph := testGraph()
	current := []SignProposal{{SignID: "S1", NodeID: "J", DestinationRefs: []string{"D"}, Direction: "right", DisplayText: "服务 台", VisibilityDistanceM: 8}, {SignID: "S2", NodeID: "D", DestinationRefs: []string{"D"}, Direction: "arrive", DisplayText: "到达", VisibilityDistanceM: 5}}
	candidate := []SignProposal{{SignID: "S1", NodeID: "J", DestinationRefs: []string{"D"}, Direction: "left", DisplayText: " 服务   台 ", VisibilityDistanceM: 8}, {SignID: "S3", NodeID: "D", DestinationRefs: []string{"D"}, Direction: "arrive", DisplayText: "到达", VisibilityDistanceM: 5}}
	preflight := PreflightSigns(graph, current, candidate)
	if len(preflight.Errors) != 0 || len(preflight.Diff.Added) != 1 || len(preflight.Diff.Modified) != 1 || len(preflight.Diff.Deleted) != 1 {
		t.Fatalf("三向差异不正确: %+v", preflight)
	}
	candidate[1].DestinationRefs = nil
	invalid := PreflightSigns(graph, current, candidate)
	if len(invalid.Errors) == 0 || invalid.Errors[0].SignID != "S3" {
		t.Fatalf("无效条目应按 sign_id 定位: %+v", invalid.Errors)
	}
}

func TestEvidenceVersionInvalidatedByAffectedSignScope(t *testing.T) {
	p := LayoutProject{ProjectID: "evidence", Survey: testGraph(), SignsRevision: 2}
	p.Issues = p.ValidateRules()
	if len(p.Issues) == 0 {
		t.Fatal("测试图应在无标识时产生问题")
	}
	issue := p.Issues[0]
	if err := p.AppendResolution(issue.IssueID, "补充标识", "照片 1", "designer", nowPtr().UTC(), p.SignsRevision); err != nil {
		t.Fatal(err)
	}
	p.InvalidateEvidence(map[string]bool{issue.NodeID: true}, map[string]bool{}, "同节点标识已修改")
	latest := p.Issues[0].EvidenceHistory[0]
	if latest.Valid || latest.InvalidReason == "" || p.Issues[0].Status != IssueOpen {
		t.Fatalf("旧证据没有进入待重新确认状态: %+v", p.Issues[0])
	}
}
