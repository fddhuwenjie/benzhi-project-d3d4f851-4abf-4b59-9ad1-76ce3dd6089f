package application

import "wayfinding-release-gate/internal/domain"

type CreateProjectCommand struct {
	Meta         CommandMeta `json:"meta"`
	BuildingName string      `json:"building_name"`
	Zones        []string    `json:"zones"`
	DesignerID   string      `json:"designer_id"`
	ReviewerID   string      `json:"reviewer_id"`
}
type FreezeBaselineCommand struct {
	Meta   CommandMeta        `json:"meta"`
	Survey domain.SurveyGraph `json:"survey"`
}
type BaselinePreflightCommand struct {
	Survey domain.SurveyGraph `json:"survey"`
}
type ReplaceSignsCommand struct {
	Meta  CommandMeta           `json:"meta"`
	Signs []domain.SignProposal `json:"signs"`
}
type SignPreflightCommand struct {
	Signs []domain.SignProposal `json:"signs"`
}
type ValidateCommand struct {
	Meta CommandMeta `json:"meta"`
}
type ResolveIssueCommand struct {
	Meta         CommandMeta           `json:"meta"`
	IssueID      string                `json:"issue_id"`
	Resolution   string                `json:"resolution"`
	Evidence     string                `json:"evidence"`
	UpdatedSigns []domain.SignProposal `json:"updated_signs,omitempty"`
}
type ReverifyCommand struct {
	Meta CommandMeta `json:"meta"`
}
type WalkthroughCommand struct {
	Meta        CommandMeta         `json:"meta"`
	Checkpoints []domain.Checkpoint `json:"checkpoints"`
}
type FreezePackageCommand struct {
	Meta            CommandMeta `json:"meta"`
	SourceRevision  int         `json:"source_revision,omitempty"`
	CandidateDigest string      `json:"candidate_digest,omitempty"`
}
type MutationResult struct {
	ProjectID         string                      `json:"project_id"`
	Revision          int                         `json:"revision"`
	Status            domain.ProjectStatus        `json:"status"`
	Message           string                      `json:"message"`
	Issues            []domain.ValidationIssue    `json:"issues,omitempty"`
	Route             []domain.Checkpoint         `json:"route,omitempty"`
	Package           *domain.InstallationPackage `json:"package,omitempty"`
	BaselinePreflight *domain.BaselinePreflight   `json:"baseline_preflight,omitempty"`
	SignPreflight     *domain.SignPreflight       `json:"sign_preflight,omitempty"`
	ApprovalPreflight *domain.ApprovalPreflight   `json:"approval_preflight,omitempty"`
}
