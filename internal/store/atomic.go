package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func atomicJSON(path string, v any) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".pending-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(v); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, 0640); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err = d.Sync(); err != nil {
		return fmt.Errorf("同步目录: %w", err)
	}
	ok = true
	return nil
}
func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
