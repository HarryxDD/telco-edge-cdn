package main

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

// load videos.js file into VideoStore
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
		return nil // treat as empty list
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

// Save data to videos.json
func (s *VideoStore) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.videos, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

// Get all videos metadata
func (s *VideoStore) GetAll() []VideoMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]VideoMeta, len(s.videos))
	copy(out, s.videos)
	return out
}

// Appends a new video and persists to disk
func (s *VideoStore) Add(v VideoMeta) error {
	s.mu.Lock()
	s.videos = append(s.videos, v)
	s.mu.Unlock()
	return s.save()
}

// Update duration of a video by name
func (s *VideoStore) UpdateDuration(title string, duration float64) error {
	var found bool
	s.mu.Lock()

	for i, v := range s.videos {
		if v.Title == title {
			s.videos[i].Duration = duration
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		log.Printf("[debug] UpdateDuration: video with title %q not found in list", title)
		return fmt.Errorf("video with title %q not found", title)
	}

	return nil
}
