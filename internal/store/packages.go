package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

func (s *FileStore) SavePackage(ctx context.Context, p domain.InstallationPackage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !domain.VerifyPackage(p) {
		return errors.New("安装包摘要无效")
	}
	path, err := s.packagePath(p.SHA256Digest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0440)
	if os.IsExist(err) {
		var old domain.InstallationPackage
		if readErr := readJSON(path, &old); readErr != nil {
			return readErr
		}
		if old.PackageID != p.PackageID || !domain.VerifyPackage(old) {
			return errors.New("摘要文件已存在但内容不同")
		}
		return nil
	}
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	if err = d.Sync(); err != nil {
		return err
	}
	ok = true
	return nil
}
func (s *FileStore) LoadPackage(ctx context.Context, id string) (domain.InstallationPackage, error) {
	if err := ctx.Err(); err != nil {
		return domain.InstallationPackage{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matches, err := filepath.Glob(filepath.Join(s.root, "packages", "*.json"))
	if err != nil {
		return domain.InstallationPackage{}, err
	}
	for _, path := range matches {
		var p domain.InstallationPackage
		if err := readJSON(path, &p); err != nil {
			return p, err
		}
		if p.PackageID == id || p.ProjectID == id || p.SHA256Digest == id {
			return p, nil
		}
	}
	return domain.InstallationPackage{}, application.ErrNotFound
}
