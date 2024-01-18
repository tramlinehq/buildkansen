package models

import (
	"buildkansen/db"
	"buildkansen/log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	Id            int64 `gorm:"primaryKey"`
	Name          string
	Email         string    `gorm:"type:varchar(100);unique_index"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
	DeletedAt     time.Time
	Installations []Installation `gorm:"foreignKey:UserId"`
}

type Installation struct {
	Id               int64 `gorm:"primaryKey"`
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
	Id             int64 `gorm:"primaryKey"`
	Name           string
	FullName       string
	Private        bool
	InstallationId int64
	Installation   Installation `gorm:"foreignKey:InstallationId;references:Id"`
	CreatedAt      time.Time    `gorm:"autoCreateTime"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime"`
	DeletedAt      time.Time
}

type VM struct {
	Id                int64 `gorm:"primaryKey"`
	VMIPAddress       string
	GithubRunnerLabel string
	Status            string
	CreatedAt         time.Time `gorm:"autoCreateTime"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`
}

func Migrate() {
	if err := db.DB.AutoMigrate(&User{}, &Installation{}, &Repository{}, &VM{}); err != nil {
		log.Fatalf("Error migrating the database")
		panic(err)
	}
}

type models interface {
	Installation | Repository | User | VM
}

type values interface {
	int64 | string
}

func FindEntityById[U models](m U, value int64) (interface{}, error) {
	return FindEntity(m, value, "id")
}

func FindEntity[U models, V values](m U, value V, by string) (interface{}, error) {
	condition := by + " = ?"
	result := db.DB.Where(condition, value).First(&m)

	if result.Error != nil {
		return nil, result.Error
	}

	return m, nil
}

func FetchInstallationsAndRepositories(user *User) ([]Installation, []Repository) {
	db.DB.Preload("Installations").Preload("Installations.Repositories").First(&user, user.Id)

	repositories := make([]Repository, 0)

	for _, installation := range user.Installations {
		for _, repository := range installation.Repositories {
			repositories = append(repositories, repository)
		}
	}

	return user.Installations, repositories
}

func UpsertUser(id int64, name string, email string) *gorm.DB {
	u := User{Id: id, Name: name, Email: email}
	return db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email",
			"name",
		}),
	}).Create(&u)
}

func CreateVM(vmIPAddress string, runnerLabel string) *gorm.DB {
	vm := VM{VMIPAddress: vmIPAddress, GithubRunnerLabel: runnerLabel, Status: "available"}
	return db.DB.Create(&vm)
}
