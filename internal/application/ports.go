package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"wayfinding-release-gate/internal/domain"
)

var (
	ErrNotFound            = errors.New("项目不存在")
	ErrRevisionConflict    = errors.New("expected_revision 与当前修订不一致")
	ErrIdempotencyConflict = errors.New("request_id 已用于不同请求")
)

type RequestRecord struct {
	ProjectID   string          `json:"project_id"`
	RequestID   string          `json:"request_id"`
	Fingerprint string          `json:"fingerprint"`
	Response    json.RawMessage `json:"response"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}
type Event struct {
	Sequence       int64             `json:"sequence"`
	ProjectID      string            `json:"project_id"`
	Type           string            `json:"type"`
	ActorID        string            `json:"actor_id"`
	Revision       int               `json:"revision"`
	At             time.Time         `json:"at"`
	Details        map[string]string `json:"details,omitempty"`
	PreviousDigest string            `json:"previous_digest"`
	Digest         string            `json:"digest"`
}
type Repository interface {
	LoadProject(context.Context, string) (*domain.LayoutProject, error)
	ListProjects(context.Context) ([]domain.LayoutProject, error)
	FindRequest(context.Context, string, string) (*RequestRecord, error)
	SaveRequest(context.Context, string, RequestRecord) error
	Commit(context.Context, *domain.LayoutProject, RequestRecord, Event) error
	Events(context.Context, string) ([]Event, error)
	SavePackage(context.Context, domain.InstallationPackage) error
	LoadPackage(context.Context, string) (domain.InstallationPackage, error)
}

type CommandMeta struct {
	ProjectID        string `json:"project_id"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int    `json:"expected_revision"`
	ActorID          string `json:"actor_id"`
}

func (m CommandMeta) validate() error {
	if m.ProjectID == "" {
		return errors.New("project_id 不能为空")
	}
	if m.RequestID == "" {
		return errors.New("request_id 不能为空")
	}
	if m.ActorID == "" {
		return errors.New("actor_id 不能为空")
	}
	if m.ExpectedRevision < 0 {
		return errors.New("expected_revision 不能为负数")
	}
	return nil
}
