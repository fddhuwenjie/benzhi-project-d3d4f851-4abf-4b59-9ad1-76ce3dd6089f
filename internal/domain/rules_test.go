package domain

import (
	"encoding/json"
	"testing"
)

func testGraph() SurveyGraph {
	return SurveyGraph{Nodes: []Node{{ID: "E", Kind: "entrance", Accessible: true}, {ID: "J", Kind: "decision", Accessible: true}, {ID: "D", Kind: "destination", Accessible: true}}, Edges: []Edge{{From: "E", To: "J", Accessible: true}, {From: "J", To: "D", Accessible: true}}, EntranceIDs: []string{"E"}, DestinationIDs: []string{"D"}, AccessibleEdgeFlags: map[string]bool{"E->J": true, "J->D": true}}
}

func TestValidateRulesDeterministic(t *testing.T) {
	p := LayoutProject{ProjectID: "p1", Survey: testGraph(), Signs: []SignProposal{{SignID: "s1", NodeID: "J", DisplayText: "服务台"}}}
	first := p.ValidateRules()
	second := p.ValidateRules()
	if Digest(first) != Digest(second) {
		t.Fatal("相同输入产生了不同问题清单")
	}
	if len(first) != 0 {
		t.Fatalf("合格图不应产生问题: %+v", first)
	}
	p.Signs = nil
	issues := p.ValidateRules()
	if len(issues) == 0 || issues[0].IssueID == "" {
		t.Fatal("缺失连续指引应生成稳定问题 ID")
	}
}

func TestResolutionRequiresEvidenceAndRulePass(t *testing.T) {
	p := LayoutProject{ProjectID: "p2", Survey: testGraph()}
	p.Issues = p.ValidateRules()
	if len(p.Issues) == 0 {
		t.Fatal("预期有问题")
	}
	p.Signs = []SignProposal{{SignID: "s1", NodeID: "J", DisplayText: "服务台"}}
	p.ReverifyIssues()
	if p.Issues[0].Status != IssueOpen {
		t.Fatal("没有整改证据时不能关闭问题")
	}
	for i := range p.Issues {
		p.Issues[i].Resolution = "补充标识"
		p.Issues[i].EvidenceDigest = Digest("现场照片")
	}
	p.ReverifyIssues()
	for _, issue := range p.Issues {
		if issue.Status != IssueResolved {
			t.Fatalf("规则通过且证据完整后应关闭: %+v", issue)
		}
	}
}

func TestPackageTamperDetection(t *testing.T) {
	pkg := InstallationPackage{ProjectID: "p", CanonicalPayload: []byte(`{"ok":true}`)}
	pkg.SHA256Digest = Digest(json.RawMessage(pkg.CanonicalPayload))
	if !VerifyPackage(pkg) {
		t.Fatal("原始安装包应通过")
	}
	pkg.CanonicalPayload[2] = 'x'
	if VerifyPackage(pkg) {
		t.Fatal("篡改安装包不应通过")
	}
}
