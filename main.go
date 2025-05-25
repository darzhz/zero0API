package main

import (
	"log"
	"zero0Api/api"
	"zero0Api/hooks"

	"github.com/pocketbase/pocketbase"
)

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(api.SetupVideoRoutes(app))
	app.OnRecordAfterCreateSuccess("videos").BindFunc(hooks.HandleVideoUpload(app))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
