package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

type selfClient struct {
	base     string
	client   *http.Client
	sequence int
}

func (c *selfClient) post(path string, body any, out any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s 返回 %d: %s", path, resp.StatusCode, payload)
	}
	if out != nil {
		return json.Unmarshal(payload, out)
	}
	return nil
}
func (c *selfClient) put(path string, body any, out any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s 返回 %d: %s", path, resp.StatusCode, payload)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
func meta(revision int, actor string, n int) application.CommandMeta {
	return application.CommandMeta{ProjectID: "selftest-project", RequestID: fmt.Sprintf("selftest-%02d", n), ExpectedRevision: revision, ActorID: actor}
}
func runSelfTest(cfg config) error {
	temp, err := os.MkdirTemp("", "wayfinding-selftest-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	cfg.DataDir = temp
	rt, err := newRuntime(cfg)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- rt.serve() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = rt.close(ctx)
		<-done
	}()
	client := &selfClient{base: "http://" + cfg.Address, client: &http.Client{Timeout: 4 * time.Second}}
	return driveSelfTest(client)
}

func driveSelfTest(c *selfClient) error {
	var result application.MutationResult
	create := application.CreateProjectCommand{Meta: meta(0, "designer-a", 1), BuildingName: "市民中心", Zones: []string{"一层大厅"}, DesignerID: "designer-a", ReviewerID: "reviewer-b"}
	if err := c.post("/api/projects", create, &result); err != nil {
		return err
	}
	nodes := []domain.Node{{ID: "ENT", Name: "主入口", Kind: "entrance", Accessible: true}, {ID: "J1", Name: "大厅", Kind: "decision", Accessible: true}, {ID: "DEST", Name: "服务台", Kind: "destination", Accessible: true}}
	edges := []domain.Edge{{From: "ENT", To: "J1", Accessible: true}, {From: "J1", To: "DEST", Accessible: true}}
	survey := domain.SurveyGraph{Nodes: nodes, Edges: edges, EntranceIDs: []string{"ENT"}, DestinationIDs: []string{"DEST"}, AccessibleEdgeFlags: map[string]bool{"ENT->J1": true, "J1->DEST": true}}
	if err := c.post("/api/projects/selftest-project/baseline", application.FreezeBaselineCommand{Meta: meta(1, "designer-a", 2), Survey: survey}, &result); err != nil {
		return err
	}
	signs := []domain.SignProposal{{SignID: "S1", NodeID: "J1", DestinationRefs: []string{"DEST"}, Direction: "straight", DisplayText: "前往服务台", VisibilityDistanceM: 10, RevisionNote: "自检"}, {SignID: "S2", NodeID: "DEST", DestinationRefs: []string{"DEST"}, Direction: "arrive", DisplayText: "服务台到达", VisibilityDistanceM: 5, RevisionNote: "自检"}}
	if err := c.put("/api/projects/selftest-project/signs", application.ReplaceSignsCommand{Meta: meta(2, "designer-a", 3), Signs: signs}, &result); err != nil {
		return err
	}
	if err := c.post("/api/projects/selftest-project/validate", application.ValidateCommand{Meta: meta(3, "designer-a", 4)}, &result); err != nil {
		return err
	}
	if result.Status != domain.StatusReadyForWalkthrough {
		return fmt.Errorf("规则校验未进入待走查: %s", result.Status)
	}
	checks := result.Route
	for i := range checks {
		checks[i].Visible = true
		checks[i].DirectionCorrect = true
	}
	if err := c.post("/api/projects/selftest-project/walkthrough", application.WalkthroughCommand{Meta: meta(4, "reviewer-b", 5), Checkpoints: checks}, &result); err != nil {
		return err
	}
	if result.Status != domain.StatusReadyForApproval {
		return fmt.Errorf("走查未进入待批准: %s", result.Status)
	}
	if err := c.post("/api/projects/selftest-project/freeze", application.FreezePackageCommand{Meta: meta(5, "reviewer-b", 6)}, &result); err != nil {
		return err
	}
	if result.Package == nil || !domain.VerifyPackage(*result.Package) {
		return fmt.Errorf("冻结包摘要验证失败")
	}
	var verified struct {
		Valid bool `json:"valid"`
	}
	if err := c.post("/api/projects/selftest-project/package/verify", map[string]any{}, &verified); err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("HTTP 摘要验证失败")
	}
	fmt.Printf("selftest 通过：项目 %s，安装包 %s，摘要 %s\n", result.ProjectID, result.Package.PackageID, result.Package.SHA256Digest)
	return nil
}
