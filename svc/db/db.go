package db

import (
	"buildkansen/config"
	"buildkansen/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	var err error
	DB, err = gorm.Open(sqlite.Open(config.C.DbName), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to the database")
		panic(err)
	}
}
