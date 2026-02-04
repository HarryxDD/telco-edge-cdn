package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// return video list as json
func handleGetVideos(s *VideoStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		videos := s.GetAll()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(videos); err != nil {
			log.Printf("encode /api/videos: %v", err)
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// handleUpload receives a file, fires up an asynchronous ffmpeg process to
// generate HLS under hlsBaseDir
// it response once file is stored and async job is scheduled
// video metadata (name, duration set to 0) is added to videos.json
// afer encoding, the duration is updated in the background
func handleUpload(s *VideoStore, uploadDir, hlsBaseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// current limit to 1GB upload
		if err := r.ParseMultipartForm(1 << 30); err != nil {
			log.Printf("parse multipart: %v", err)
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			log.Printf("FormFile(file): %v", err)
			http.Error(w, "file field is required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		uploadPath, title, err := saveUploadFile(file, header, uploadDir)
		if err != nil {
			log.Printf("saveUploadedFile: %v", err)
			http.Error(w, "failed to save upload", http.StatusInternalServerError)
			return
		}

		// immediately add video to videoStore so it shows up on the list
		meta := VideoMeta{Title: title, Duration: 0}
		if err := s.Add(meta); err != nil {
			log.Printf("store.Add: %v", err)
			// Not fatal for the upload itself, but the frontend relies on the list.
			// Return an error to avoid confusing UX.
			http.Error(w, "failed to register video", http.StatusInternalServerError)
			return
		}

		// schedule asynchronous ffmpeg and ffprobe
		outputDir := filepath.Join(hlsBaseDir, title)
		go func(title, inPath, outDir string) {
			log.Printf("[async] start HLS generation for %s", title)
			if err := generateHLS(inPath, outDir); err != nil {
				log.Printf("[async] error in generateHLS(%s): %v", title, err)
				return
			}

			dur, err := probeDuration(inPath)
			if err != nil {
				log.Printf("[async] error in probeDuration(%s, path=%s): %v", title, inPath, err)
			} else {
				log.Printf("[async] probeDuration(%s, path=%s): duration=%.2f", title, inPath, dur)
				if err := s.UpdateDuration(title, dur); err != nil {
					log.Printf("[async] error in UpdateDuration(%s, duration=%.2f): %v", title, dur, err)
				} else {
					log.Printf("[async] updated duration for %s to %.2f seconds", title, dur)
				}
			}
		}(title, uploadPath, outputDir)

		// response processing status
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "processing",
			"title":  title,
		})

	}
}

// log basic request information
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})

}

// serve HLS playlists and segments from hlsDir
func hlsHandler(hlsDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get relative path for hls
		rel := strings.TrimPrefix(r.URL.Path, "/hls/")
		rel = filepath.Clean(rel)

		fullPath := filepath.Join(hlsDir, rel)
		http.ServeFile(w, r, fullPath)
	})
}

// wire all http handlers and wrap them with logging
func buildMux(s *VideoStore, uploadDir, hlsDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/videos", handleGetVideos(s))
	mux.HandleFunc("/api/upload", handleUpload(s, uploadDir, hlsDir))

	mux.Handle("/hls/", hlsHandler(hlsDir))

	return loggingMiddleware(mux)
}
