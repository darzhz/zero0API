package hooks

import (
	"log"
	"path/filepath"
	"zero0Api/utils"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func HandleVideoUpload(app *pocketbase.PocketBase) func(*core.RecordEvent) error {
	return func(e *core.RecordEvent) error {
		fileName := e.Record.Get("video").(string)
		e.Record.Set("status", "processing")
		go func() {
			filePath := filepath.Join("pb_data", "storage", e.Record.BaseFilesPath(), fileName)

			r480 := "https://localhost:8090/" + utils.EncodeVideo(filePath, 480, e.Record.Id)
			r720 := "https://localhost:8090/" + utils.EncodeVideo(filePath, 720, e.Record.Id)

			e.Record.Set("status", "completed")
			e.Record.Set("data", utils.EncodedVideo{R480p: r480, R720p: r720})

			if err := app.Save(e.Record); err != nil {
				log.Println("Failed to update video record:", err)
			}
		}()
		return e.Next()
	}
}
