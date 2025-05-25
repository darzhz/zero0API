package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type EncodedVideo struct {
	R720p string
	R480p string
}

func EncodeVideo(filePath string, resolution int, recordID string) string {
	ext := filepath.Ext(filePath)
	base := filepath.Base(filePath[:len(filePath)-len(ext)])
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	uniqueFileName := fmt.Sprintf("%s_%s_%dp%s", base, timestamp, resolution, ext)

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	outputDir := filepath.Join(wd, "pb_public", recordID)
	_ = os.MkdirAll(outputDir, os.ModePerm)

	outputPath := filepath.Join(outputDir, uniqueFileName)
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", filePath,
		"-vf", fmt.Sprintf("scale=-2:%d", resolution),
		"-c:v", "libx264", "-crf", "23", "-preset", "veryfast",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[FFMPEG ERROR] %s\n", output)
		return "N/A"
	}
	return filepath.ToSlash(filepath.Join("pb_public", recordID, uniqueFileName))
}
