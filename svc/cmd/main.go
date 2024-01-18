package main

import (
	"buildkansen/config"
	"buildkansen/db"
	"buildkansen/internal/jobs"
	"buildkansen/log"
	"buildkansen/models"
	"buildkansen/web"
)

func main() {
	log.Init()
	config.Load()
	db.Init()
	models.Migrate()
	jobs.Start()
	web.Run()
}
