package api

import (
	"fmt"
	"net/http"
	"os"
	"time"
	"zero0Api/utils"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func SetupVideoRoutes(app *pocketbase.PocketBase) func(*core.ServeEvent) error {
	return func(e *core.ServeEvent) error {
		// serve static files
		e.Router.GET("/pb_public/{path...}", apis.Static(os.DirFS("./pb_public"), false))
		if wd, err := os.Getwd(); err == nil {
			fmt.Println("CWD:", wd)
		} else {
			fmt.Println("Error getting CWD:", err)
		}

		// trending videos route
		e.Router.GET("/api/next-videos", func(c *core.RequestEvent) error {
			videosCollection, _ := app.FindCollectionByNameOrId("videos")

			records, _ := app.FindRecordsByFilter(
				videosCollection,
				"",           // all videos
				"-heatScore", // sort by trending
				20,           // limit
				0,
			)
			errs := app.ExpandRecords(records, []string{"uploadedBy"}, nil)
			if len(errs) > 0 {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to expand records",
				})
			}

			nowPlaying := map[string]any{}
			prefetch := []map[string]any{}

			if len(records) > 0 {
				nowPlaying = records[0].PublicExport()
				end := 4
				if len(records) < 5 {
					end = len(records)
				}
				prefetch = utils.ToPublicList(records[1:end])
			}

			return c.JSON(http.StatusOK, map[string]any{
				"nowPlaying": nowPlaying,
				"prefetch":   prefetch,
			})
		})
		e.Router.GET("/api/register/{type}/{id}", func(c *core.RequestEvent) error {
			registerType := c.Request.PathValue("type")
			id := c.Request.PathValue("id")

			videosCollection, _ := app.FindCollectionByNameOrId("videos")
			record, _ := app.FindRecordById(videosCollection, id)

			if registerType == "like" && record.Get("likes").(float64) >= 0 {
				record.Set("likes", record.Get("likes").(float64)+1)
			} else if registerType == "dislike" && record.Get("likes").(float64) > 0 {
				record.Set("likes", record.Get("likes").(float64)-1)
			} else if registerType == "view" {
				record.Set("views", record.Get("views").(float64)+1)
			}
			//calculate heat score based on likes, views and created date
			created := record.Get("created").(types.DateTime).Time()
			heatScore := record.Get("likes").(float64)*0.2 +
				record.Get("views").(float64)*0.3 +
				float64(time.Since(created).Hours()/24)
			record.Set("heatScore", heatScore)

			if err := app.Save(record); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{
					"error": "Failed to save record",
				})
			}

			return c.JSON(http.StatusOK, record)
		})

		return e.Next()
	}
}
