package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		return se.Next()
	})
	//hook for video upload
	app.OnRecordCreateRequest("videos").BindFunc(func(e *core.RecordRequestEvent) error {
		fileName := e.Record.GetUnsavedFiles("video")[0].Name
		go handleNewUpload(e.Record, fileName) // run in background
		return e.Next()

	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func handleNewUpload(record *core.Record, fileName string) {
	// 1. Get uploaded file path
	filePath := filepath.Join("pb_data", "storage", record.BaseFilesPath(), fileName)

	// 2. Encode into 480p and 720p using FFmpeg
	encodeVideo(filePath, 480)
	encodeVideo(filePath, 720)
	fmt.Println("video uploaded")
	fmt.Println("encoding started at", filePath)
	// 3. Update the record with encoding status
	// db := app.Dao()
	// record.Set("encoding_status", "completed") // or "failed" if errors
	// if err := db.SaveRecord(record); err != nil {
	//     log.Println("Failed to update video record:", err)
	// }
}

func encodeVideo(filePath string, resolution int) {
	// Get the video file extension
	ext := filepath.Ext(filePath)
	log.Println("extension", ext)
	log.Println("file path", filePath)

	// Generate the output file path
	outputPath := fmt.Sprintf("%s_%dp%s", filePath, resolution, ext)

	// Set up the command
	cmd := exec.Command("ffmpeg", "-i", filePath, "-s", fmt.Sprintf("%dx%d", resolution, resolution), "-c:v", "libx264", "-crf", "18", outputPath)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(err)
		log.Println(string(output))
		return
	}

	// Log the output
	log.Printf("Encoded video %s to %s", filePath, outputPath)
}
