package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// convert file name to a safe title used for URLs
func sanitizeTitle(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	name = strings.ToLower(strings.TrimSpace(name))
	// replace invalid characters with dash
	re := regexp.MustCompile(`[^a-z0-9_-]+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	if name == "" {
		name = "video"
	}

	return fmt.Sprint("%s-%d", name, time.Now().Unix())
}

func saveUploadFile(file multipart.File, header *multipart.FileHeader, uploadsDir string) (string, string, error) {
	if err := os.Mkdir(uploadsDir, 0755); err != nil {
		return "", "", err
	}
	title := sanitizeTitle(header.Filename)
	ext := filepath.Ext(header.Filename)

	// default extension is mp4
	if ext == "" {
		ext = ".mp4"
	}

	filename := title + ext
	outPath := filepath.Join(uploadsDir, filename)

	out, err := os.Create(outPath)
	if err != nil {
		return "", "", err
	}

	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", "", err
	}
	return outPath, title, nil
}

// handle hls generation process using ffmpeg (asynchronous)
// Ensure to have ffmpeg installed on machine
func generateHLS(inputPath, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	args := []string{
		"-y", // overwrite output
		"-i", inputPath,
		"-preset", "veryfast",
		"-g", "48",
		"-keyint_min", "48",
		"-sc_threshold", "0",
		"-c:v", "h264",
		"-c:a", "aac",
		"-f", "hls",
		"-hls_time", "1", // ~1 second segments
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

// probe duration using ffprobe
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
