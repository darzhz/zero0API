package main

import (
	"log"
	"zero0Api/api"
	"zero0Api/hooks"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	app := pocketbase.New()

	app.OnServe().BindFunc(api.SetupVideoRoutes(app))
	app.OnServe().BindFunc(api.SetupPaymentsRoutes(app))
	app.OnRecordAfterCreateSuccess("videos").BindFunc(hooks.HandleVideoUpload(app))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
