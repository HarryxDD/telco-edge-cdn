package main

import "net/http"

// return video list as json
func handleGetVideos(s *VideoStore) http.HandlerFunc {

}

// handleUpload receives a file, fires up an asynchronous ffmpeg process to
// generate HLS under hlsBaseDir
// it response once file is stored and async job is scheduled
// video metadata (name, duration set to 0) is added to videos.json
// afer encoding, the duration is updated in the background
func handleUpload(s *VideoStore, uploadDir, hlsBaseDir string) http.HandlerFunc {

}

// log basic request information
func loggingMiddleware(next http.Handler) http.Handler {

}

// serve HLS playlists and segments from hlsDir
func hlsHandler(hlsDir string) http.Handler {

}

// wire all http handlers and wrap them with logging
func buildMux(s *VideoStore, uploadDir, hlsDir string) http.Handler {
	
}

