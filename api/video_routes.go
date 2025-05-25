package api

import (
	"net/http"
	"os"
	"zero0Api/utils"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func SetupVideoRoutes(app *pocketbase.PocketBase) func(*core.ServeEvent) error {
	return func(e *core.ServeEvent) error {
		// serve static files
		e.Router.GET("pb_public/{path...}", apis.Static(os.DirFS("./pb_public"), false))

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

		return e.Next()
	}
}
