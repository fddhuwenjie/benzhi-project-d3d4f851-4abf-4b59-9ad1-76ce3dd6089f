package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ProjectStatus string

const (
	StatusDraft               ProjectStatus = "draft"
	StatusBaselineFrozen      ProjectStatus = "baseline_frozen"
	StatusNeedsRemediation    ProjectStatus = "needs_remediation"
	StatusReadyForWalkthrough ProjectStatus = "ready_for_walkthrough"
	StatusWalkthroughFailed   ProjectStatus = "walkthrough_failed"
	StatusReadyForApproval    ProjectStatus = "ready_for_approval"
	StatusFrozen              ProjectStatus = "frozen"
)

type Node struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Accessible bool   `json:"accessible"`
}

type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Accessible bool   `json:"accessible"`
}

type SurveyGraph struct {
	Nodes               []Node          `json:"nodes"`
	Edges               []Edge          `json:"edges"`
	EntranceIDs         []string        `json:"entrance_ids"`
	DestinationIDs      []string        `json:"destination_ids"`
	AccessibleEdgeFlags map[string]bool `json:"accessible_edge_flags"`
	BaselineDigest      string          `json:"baseline_digest"`
}

type SignProposal struct {
	SignID              string   `json:"sign_id"`
	NodeID              string   `json:"node_id"`
	DestinationRefs     []string `json:"destination_refs"`
	Direction           string   `json:"direction"`
	DisplayText         string   `json:"display_text"`
	VisibilityDistanceM float64  `json:"visibility_distance_m"`
	RevisionNote        string   `json:"revision_note"`
}

type IssueStatus string

const (
	IssueOpen     IssueStatus = "open"
	IssueResolved IssueStatus = "resolved"
)

type ValidationIssue struct {
	IssueID         string            `json:"issue_id"`
	RuleCode        string            `json:"rule_code"`
	NodeID          string            `json:"node_id"`
	Severity        string            `json:"severity"`
	Status          IssueStatus       `json:"status"`
	Resolution      string            `json:"resolution"`
	EvidenceDigest  string            `json:"evidence_digest"`
	VerifiedAt      *time.Time        `json:"verified_at,omitempty"`
	Impact          IssueImpact       `json:"impact"`
	EvidenceHistory []EvidenceVersion `json:"evidence_history,omitempty"`
	ReverifyReasons []string          `json:"reverify_reasons,omitempty"`
}

type IssueImpact struct {
	EntranceIDs      []string   `json:"entrance_ids"`
	DestinationIDs   []string   `json:"destination_ids"`
	Paths            [][]string `json:"paths"`
	CheckedEntrances []string   `json:"checked_entrances,omitempty"`
}

type EvidenceVersion struct {
	Version        int        `json:"version"`
	Resolution     string     `json:"resolution"`
	Evidence       string     `json:"evidence"`
	EvidenceDigest string     `json:"evidence_digest"`
	SubmittedBy    string     `json:"submitted_by"`
	SubmittedAt    time.Time  `json:"submitted_at"`
	SignRevision   int        `json:"sign_revision"`
	Valid          bool       `json:"valid"`
	InvalidReason  string     `json:"invalid_reason,omitempty"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
}

type Checkpoint struct {
	ID               string   `json:"id"`
	NodeID           string   `json:"node_id"`
	Visible          bool     `json:"visible"`
	DirectionCorrect bool     `json:"direction_correct"`
	Note             string   `json:"note"`
	FailedDimensions []string `json:"failed_dimensions,omitempty"`
}

type WalkthroughReview struct {
	ReviewID            string       `json:"review_id"`
	ReviewerID          string       `json:"reviewer_id"`
	RouteSeed           string       `json:"route_seed"`
	Checkpoints         []Checkpoint `json:"checkpoints"`
	FailedCheckpointIDs []string     `json:"failed_checkpoint_ids"`
	SignedAt            *time.Time   `json:"signed_at,omitempty"`
	Decision            string       `json:"decision"`
	ParentReviewID      string       `json:"parent_review_id,omitempty"`
	Targeted            bool         `json:"targeted"`
	Round               int          `json:"round"`
	BaselineDigest      string       `json:"baseline_digest"`
	SignRevision        int          `json:"sign_revision"`
}

type InstallationPackage struct {
	PackageID        string    `json:"package_id"`
	ProjectID        string    `json:"project_id"`
	SourceRevision   int       `json:"source_revision"`
	CanonicalPayload []byte    `json:"canonical_payload"`
	SHA256Digest     string    `json:"sha256_digest"`
	ApprovedBy       string    `json:"approved_by"`
	FrozenAt         time.Time `json:"frozen_at"`
}

type LayoutProject struct {
	ProjectID                string               `json:"project_id"`
	BuildingName             string               `json:"building_name"`
	Zones                    []string             `json:"zones"`
	DesignerID               string               `json:"designer_id"`
	ReviewerID               string               `json:"reviewer_id"`
	Status                   ProjectStatus        `json:"status"`
	Revision                 int                  `json:"revision"`
	BaselineFrozenAt         *time.Time           `json:"baseline_frozen_at,omitempty"`
	CreatedAt                time.Time            `json:"created_at"`
	Survey                   SurveyGraph          `json:"survey"`
	Signs                    []SignProposal       `json:"signs"`
	SignsRevision            int                  `json:"signs_revision"`
	ValidatedSignsRevision   int                  `json:"validated_signs_revision"`
	Issues                   []ValidationIssue    `json:"issues"`
	IssueSummary             IssueSummary         `json:"issue_summary"`
	LastValidatedAt          *time.Time           `json:"last_validated_at,omitempty"`
	Walkthroughs             []WalkthroughReview  `json:"walkthroughs"`
	WalkthroughRoute         []Checkpoint         `json:"walkthrough_route,omitempty"`
	WalkthroughRemediatedIDs []string             `json:"walkthrough_remediated_checkpoint_ids,omitempty"`
	Package                  *InstallationPackage `json:"package,omitempty"`
}

func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func NormalizeGraph(g SurveyGraph) SurveyGraph {
	for i := range g.Nodes {
		g.Nodes[i].ID = strings.TrimSpace(g.Nodes[i].ID)
		g.Nodes[i].Name = strings.TrimSpace(g.Nodes[i].Name)
		g.Nodes[i].Kind = strings.ToLower(strings.TrimSpace(g.Nodes[i].Kind))
	}
	for i := range g.Edges {
		g.Edges[i].From = strings.TrimSpace(g.Edges[i].From)
		g.Edges[i].To = strings.TrimSpace(g.Edges[i].To)
	}
	for i := range g.EntranceIDs {
		g.EntranceIDs[i] = strings.TrimSpace(g.EntranceIDs[i])
	}
	for i := range g.DestinationIDs {
		g.DestinationIDs[i] = strings.TrimSpace(g.DestinationIDs[i])
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From == g.Edges[j].From {
			return g.Edges[i].To < g.Edges[j].To
		}
		return g.Edges[i].From < g.Edges[j].From
	})
	sort.Strings(g.EntranceIDs)
	sort.Strings(g.DestinationIDs)
	if g.AccessibleEdgeFlags == nil {
		g.AccessibleEdgeFlags = map[string]bool{}
	}
	return g
}

func (g SurveyGraph) ComputeDigest() string {
	n := NormalizeGraph(g)
	n.BaselineDigest = ""
	return Digest(n)
}

func (p *LayoutProject) ValidateIdentity() error {
	if strings.TrimSpace(p.ProjectID) == "" || strings.TrimSpace(p.BuildingName) == "" {
		return errors.New("项目和建筑名称不能为空")
	}
	if p.DesignerID == "" || p.ReviewerID == "" {
		return errors.New("设计员和复核员不能为空")
	}
	if p.DesignerID == p.ReviewerID {
		return errors.New("设计员与复核员必须不同")
	}
	return nil
}

func (p *LayoutProject) EnsureEditable() error {
	if p.Status == StatusFrozen {
		return errors.New("项目已冻结，禁止修改")
	}
	return nil
}

func (p *LayoutProject) Transition(next ProjectStatus) error {
	allowed := map[ProjectStatus]map[ProjectStatus]bool{
		StatusDraft:               {StatusBaselineFrozen: true},
		StatusBaselineFrozen:      {StatusNeedsRemediation: true, StatusReadyForWalkthrough: true},
		StatusNeedsRemediation:    {StatusNeedsRemediation: true, StatusReadyForWalkthrough: true},
		StatusReadyForWalkthrough: {StatusWalkthroughFailed: true, StatusReadyForApproval: true},
		StatusWalkthroughFailed:   {StatusNeedsRemediation: true, StatusReadyForWalkthrough: true, StatusReadyForApproval: true},
		StatusReadyForApproval:    {StatusFrozen: true, StatusWalkthroughFailed: true},
		StatusFrozen:              {},
	}
	if !allowed[p.Status][next] {
		return fmt.Errorf("不允许从 %s 转为 %s", p.Status, next)
	}
	p.Status = next
	return nil
}
