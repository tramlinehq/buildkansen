package db

import (
	"buildkansen/config"
	"buildkansen/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	var err error
	DB, err = gorm.Open(postgres.Open(config.C.DbConnectionString), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to the database")
		panic(err)
	}
}
