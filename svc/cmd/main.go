package main

import (
	"buildkansen/config"
	"buildkansen/db"
	"buildkansen/log"
	"buildkansen/models"
	"buildkansen/web"
)

func main() {
	log.Init()
	config.Load()
	db.Init()
	models.Migrate()
	web.Run()
}
