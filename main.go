package main

import (
	"fmt"
	"log"
	"os"
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
		go handleNewUpload(e.Record) // run in background
		return e.Next()

	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func handleNewUpload(record *core.Record) {
	// 1. Get uploaded file path
	filePath := filepath.Join("pb_data", "storage", record.BaseFilesPath(), record.GetString("video"))

	// 2. Encode into 480p and 720p using FFmpeg
	// encodeVideo(filePath, 480)
	// encodeVideo(filePath, 720)
	fmt.Println("video uploaded")
	fmt.Println("encoding started at", filePath)
	// 3. Update the record with encoding status
	// db := app.Dao()
	// record.Set("encoding_status", "completed") // or "failed" if errors
	// if err := db.SaveRecord(record); err != nil {
	//     log.Println("Failed to update video record:", err)
	// }
}
