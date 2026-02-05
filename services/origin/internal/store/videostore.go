package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

type VideoStore struct {
	mu     sync.RWMutex
	path   string
	videos []VideoMeta
}

type VideoMeta struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
}

func NewVideoStore(path string) (*VideoStore, error) {
	s := &VideoStore{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *VideoStore) load() error {
	f, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if len(data) == 0 || len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var videos []VideoMeta
	if err := json.Unmarshal(data, &videos); err != nil {
		return err
	}

	s.mu.Lock()
	s.videos = videos
	s.mu.Unlock()
	return nil
}

func (s *VideoStore) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.videos, "", " ")
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func (s *VideoStore) GetAll() []VideoMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]VideoMeta, len(s.videos))
	copy(out, s.videos)
	return out
}

func (s *VideoStore) Add(v VideoMeta) error {
	s.mu.Lock()
	s.videos = append(s.videos, v)
	s.mu.Unlock()
	return s.save()
}

func (s *VideoStore) UpdateDuration(title string, duration float64) error {
	s.mu.Lock()
	found := false
	for i, v := range s.videos {
		if v.Title == title {
			s.videos[i].Duration = duration
			found = true
			break
		}
	}
	s.mu.Unlock()

	if !found {
		log.Printf("[WARN] UpdateDuration: video %q not found", title)
		return fmt.Errorf("video %q not found", title)
	}

	return s.save()
}
