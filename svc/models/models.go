package models

import (
	"buildkansen/db"
	"buildkansen/log"
	"time"
)

type User struct {
	Id            int64 `gorm:"primary_key"`
	Name          string
	Email         string    `gorm:"type:varchar(100);unique_index"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
	DeletedAt     time.Time
	Installations []Installation `gorm:"foreignKey:UserId"`
}

type Installation struct {
	Id               int64 `gorm:"primary_key"`
	AccountType      string
	AccountID        int64
	AccountLogin     string
	AccountAvatarUrl string
	UserId           int64
	User             User         `gorm:"foreignKey:UserId;references:Id"`
	Repositories     []Repository `gorm:"foreignKey:InstallationId"`
	CreatedAt        time.Time    `gorm:"autoCreateTime"`
	UpdatedAt        time.Time    `gorm:"autoUpdateTime"`
	DeletedAt        time.Time
}

type Repository struct {
	Id             int64 `gorm:"primary_key"`
	Name           string
	FullName       string
	Private        bool
	InstallationId int64
	Installation   Installation `gorm:"foreignKey:InstallationId;references:Id"`
	CreatedAt      time.Time    `gorm:"autoCreateTime"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime"`
	DeletedAt      time.Time
}

func Migrate() {
	if err := db.DB.AutoMigrate(&User{}, &Installation{}, &Repository{}); err != nil {
		log.Fatalf("Error migrating the database")
		panic(err)
	}
}

type model interface {
	Installation | Repository | User
}

func FindEntityById[U model](m U, value int64) (interface{}, error) {
	return FindEntity(m, value, "id")
}

func FindEntity[U model](m U, value int64, by string) (interface{}, error) {
	condition := by + " = ?"
	result := db.DB.Where(condition, value).First(&m)

	if result.Error != nil {
		return nil, result.Error
	}

	return m, nil
}
