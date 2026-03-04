package service

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/origin/internal/store"
)

type Encoder struct {
	uploadsDir string
	hlsDir     string
	store      *store.VideoStore
}

func NewEncoder(uploadsDir, hlsDir string, store *store.VideoStore) *Encoder {
	return &Encoder{
		uploadsDir: uploadsDir,
		hlsDir:     hlsDir,
		store:      store,
	}
}

func (e *Encoder) ProcessUpload(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Generate safe title
	title := sanitizeTitle(fileHeader.Filename)
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".mp4"
	}

	// Save uploaded file
	uploadPath := filepath.Join(e.uploadsDir, title+ext)
	out, err := os.Create(uploadPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}

	// Add to store immediately (duration = 0)
	meta := store.VideoMeta{Title: title, Duration: 0}
	if err := e.store.Add(meta); err != nil {
		return "", err
	}

	// Schedule async encoding
	outputDir := filepath.Join(e.hlsDir, title)
	go e.encodeVideo(title, uploadPath, outputDir)

	return title, nil
}

func (e *Encoder) encodeVideo(title, inputPath, outputDir string) {
	log.Printf("[ENCODER] Starting HLS encoding for %s", title)

	// Generate HLS
	if err := generateHLS(inputPath, outputDir); err != nil {
		log.Printf("[ENCODER] Error generating HLS for %s: %v", title, err)
		return
	}

	// Probe duration
	duration, err := probeDuration(inputPath)
	if err != nil {
		log.Printf("[ENCODER] Error probing duration for %s: %v", title, err)
	} else {
		if err := e.store.UpdateDuration(title, duration); err != nil {
			log.Printf("[ENCODER] Error updating duration for %s: %v", title, err)
		} else {
			log.Printf("[ENCODER] ✓ Completed encoding for %s (duration: %.2fs)", title, duration)
		}
	}
}

func sanitizeTitle(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	name = strings.ToLower(strings.TrimSpace(name))
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	if name == "" {
		name = "video"
	}

	return fmt.Sprintf("%s-%d", name, time.Now().Unix())
}

func generateHLS(inputPath, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	args := []string{
		"-y",
		"-i", inputPath,
		"-preset", "veryfast",
		"-g", "48",
		"-keyint_min", "48",
		"-sc_threshold", "0",
		"-c:v", "h264",
		"-c:a", "aac",
		"-f", "hls",
		"-hls_time", "1",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", filepath.Join(outputDir, "segment_%04d.m4s"),
		"-master_pl_name", "master.m3u8",
		"-hls_flags", "independent_segments+split_by_time+temp_file+append_list+program_date_time",
		filepath.Join(outputDir, "stream_0.m3u8"),
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func probeDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	text := strings.TrimSpace(string(output))
	if text == "" {
		return 0, fmt.Errorf("empty ffprobe output")
	}

	dur, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, err
	}
	return dur, nil
}
