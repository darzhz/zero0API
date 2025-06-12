package main

import (
	"log"
	"zero0Api/api"
	"zero0Api/hooks"
	"zero0Api/utils"

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

	utils.InitPolling(5) // Initialize polling with 5 workers

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
