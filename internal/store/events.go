package store

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"wayfinding-release-gate/internal/application"
	"wayfinding-release-gate/internal/domain"
)

type eventFrame struct {
	Length int               `json:"length"`
	Event  application.Event `json:"event"`
}

func eventDigest(e application.Event) string { cp := e; cp.Digest = ""; return domain.Digest(cp) }
func encodeFrame(e application.Event) ([]byte, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	f := eventFrame{Length: len(payload), Event: e}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (s *FileStore) appendEventLocked(e application.Event) error {
	events, err := s.readEventsUnlocked(e.ProjectID)
	if err != nil && !errors.Is(err, application.ErrNotFound) {
		return err
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		e.Sequence = last.Sequence + 1
		e.PreviousDigest = last.Digest
	} else {
		e.Sequence = 1
		e.PreviousDigest = ""
	}
	e.Digest = eventDigest(e)
	frame, err := encodeFrame(e)
	if err != nil {
		return err
	}
	path, err := s.eventPath(e.ProjectID)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	if _, err = f.Write(frame); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (s *FileStore) readEventsUnlocked(id string) ([]application.Event, error) {
	path, err := s.eventPath(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, mapNotFound(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	out := []application.Event{}
	previous := ""
	var seq int64 = 1
	for scanner.Scan() {
		raw := scanner.Bytes()
		var frame eventFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			return nil, fmt.Errorf("事件帧 JSON: %w", err)
		}
		payload, _ := json.Marshal(frame.Event)
		if frame.Length != len(payload) {
			return nil, fmt.Errorf("事件帧长度不符: %d != %d", frame.Length, len(payload))
		}
		if frame.Event.Sequence != seq {
			return nil, fmt.Errorf("事件序号不连续: %d", frame.Event.Sequence)
		}
		if frame.Event.PreviousDigest != previous {
			return nil, errors.New("事件前序摘要不一致")
		}
		if eventDigest(frame.Event) != frame.Event.Digest {
			return nil, errors.New("事件摘要不一致")
		}
		previous = frame.Event.Digest
		seq++
		out = append(out, frame.Event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *FileStore) Events(ctx context.Context, id string) ([]application.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readEventsUnlocked(id)
}
func (s *FileStore) verifyAllEvents() error {
	matches, err := filepath.Glob(filepath.Join(s.root, "events", "*.frames"))
	if err != nil {
		return err
	}
	for _, path := range matches {
		id := strings.TrimSuffix(filepath.Base(path), ".frames")
		if _, err := s.readEventsUnlocked(id); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	return nil
}

func ParseFrameLength(line string) (int, error) {
	var f eventFrame
	if err := json.Unmarshal([]byte(line), &f); err != nil {
		return 0, err
	}
	return f.Length, nil
}
func ReadExact(r io.Reader, n int) ([]byte, error) {
	if n < 0 {
		return nil, errors.New("负长度")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
func FormatSequence(n int64) string { return strconv.FormatInt(n, 10) }
