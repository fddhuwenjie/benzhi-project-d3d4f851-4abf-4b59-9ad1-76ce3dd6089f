package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"wayfinding-release-gate/internal/application"
)

type FileStore struct {
	root string
	mu   sync.RWMutex
}

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func Open(root string) (*FileStore, error) {
	if root == "" {
		return nil, errors.New("数据目录不能为空")
	}
	s := &FileStore{root: root}
	for _, d := range []string{"projects", "events", "packages"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0750); err != nil {
			return nil, err
		}
	}
	if err := s.verifyAllEvents(); err != nil {
		return nil, fmt.Errorf("事件链校验失败: %w", err)
	}
	return s, nil
}
func (s *FileStore) projectPath(id string) (string, error) {
	if !safeID.MatchString(id) {
		return "", errors.New("无效 project_id")
	}
	return filepath.Join(s.root, "projects", id+".json"), nil
}
func (s *FileStore) eventPath(id string) (string, error) {
	if !safeID.MatchString(id) {
		return "", errors.New("无效 project_id")
	}
	return filepath.Join(s.root, "events", id+".frames"), nil
}
func (s *FileStore) packagePath(digest string) (string, error) {
	if len(digest) != 64 || !safeID.MatchString(digest) {
		return "", errors.New("无效摘要")
	}
	return filepath.Join(s.root, "packages", digest+".json"), nil
}
func mapNotFound(err error) error {
	if os.IsNotExist(err) {
		return application.ErrNotFound
	}
	return err
}
func (s *FileStore) Close() error                 { return nil }
func (s *FileStore) Health(context.Context) error { _, err := os.Stat(s.root); return err }
