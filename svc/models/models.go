package models

import (
	"buildkansen/db"
	"buildkansen/log"
	"database/sql"
	"errors"
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
	Installations []Installation `gorm:"foreignKey:UserId;constraint:OnDelete:CASCADE"`
}

type Installation struct {
	InternalId       int64 `gorm:"primaryKey"`
	Id               int64 `gorm:"index:idx_uniq_installation,unique"`
	AccountType      string
	AccountID        int64
	AccountLogin     string
	AccountAvatarUrl string
	UserId           int64        `gorm:"index:idx_uniq_installation,unique"`
	User             User         `gorm:"foreignKey:UserId;references:Id"`
	Repositories     []Repository `gorm:"foreignKey:InstallationId;constraint:OnDelete:CASCADE"`
	CreatedAt        time.Time    `gorm:"autoCreateTime"`
	UpdatedAt        time.Time    `gorm:"autoUpdateTime"`
	DeletedAt        time.Time
}

type Repository struct {
	InternalId      int64 `gorm:"primaryKey"`
	Id              int64 `gorm:"index:idx_uniq_repository,unique"`
	Name            string
	FullName        string
	Private         bool
	InstallationId  int64            `gorm:"index:idx_uniq_repository,unique"`
	Installation    Installation     `gorm:"foreignKey:InstallationId;references:InternalId"`
	WorkflowJobRuns []WorkflowJobRun `gorm:"foreignKey:RepositoryId;constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time        `gorm:"autoCreateTime"`
	UpdatedAt       time.Time        `gorm:"autoUpdateTime"`
	DeletedAt       time.Time
}

type WorkflowJobRun struct {
	InternalId    int64 `gorm:"primaryKey"`
	Id            int64
	Name          string
	Url           string
	WorkflowRunId int64
	WorkflowName  string
	Status        string
	Conclusion    sql.NullString
	RepositoryId  int64
	Repository    Repository `gorm:"foreignKey:RepositoryId;references:InternalId"`
	CreatedAt     time.Time  `gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
	StartedAt     time.Time
	KickoffAt     sql.NullTime
	ProcessingAt  sql.NullTime
	EndedAt       sql.NullTime
	RunDuration   time.Duration `gorm:"-"`
	QueueDuration time.Duration `gorm:"-"`
}

type VMStatus string

const (
	VMAvailable  VMStatus = "available"
	VMProcessing VMStatus = "processing"
)

type VM struct {
	Id                int64 `gorm:"primaryKey"`
	VMIPAddress       string
	VMInstanceName    string
	BaseVMName        string
	GithubRunnerLabel string
	ExternalRunId     sql.NullInt64
	RepositoryId      sql.NullInt64
	Repository        Repository `gorm:"foreignKey:RepositoryId;references:InternalId"`
	Status            VMStatus   `sql:"type:enum('available', 'processing')"`
	CreatedAt         time.Time  `gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime"`
}

func Migrate() {
	if err := db.DB.AutoMigrate(&User{}, &Installation{}, &Repository{}, &VM{}, &WorkflowJobRun{}); err != nil {
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

func FindRepositoryByInstallation(installationId int64, repositoryId int64) (*Repository, error) {
	repository := Repository{}
	result := db.DB.Model(&repository).Where("id = ? AND installation_id = ?", repositoryId, installationId).First(&repository)

	if result.Error != nil {
		return nil, result.Error
	}

	return &repository, nil
}

func FetchUserData(user *User) ([]Installation, []Repository, []WorkflowJobRun) {
	db.DB.Preload("Installations.Repositories.WorkflowJobRuns", func(db *gorm.DB) *gorm.DB {
		return db.Order("workflow_job_runs.created_at DESC").Limit(20)
	}).Preload("Installations.Repositories.WorkflowJobRuns.Repository").Preload(clause.Associations).First(&user, user.Id)

	repositories := make([]Repository, 0)
	runs := make([]WorkflowJobRun, 0)

	for _, installation := range user.Installations {
		for _, repository := range installation.Repositories {
			repositories = append(repositories, repository)
			for _, workflowJobRun := range repository.WorkflowJobRuns {
				executionTime, queueTime := time.Duration(0), time.Duration(0)

				if workflowJobRun.ProcessingAt.Valid {
					queueTime = workflowJobRun.ProcessingAt.Time.Sub(workflowJobRun.StartedAt)
					processingAt := workflowJobRun.ProcessingAt.Time

					if workflowJobRun.EndedAt.Valid {
						// job has ended
						executionTime = workflowJobRun.EndedAt.Time.Sub(processingAt)
					} else {
						// job is still running
						executionTime = time.Now().Sub(processingAt)
					}
				} else {
					queueTime = time.Now().Sub(workflowJobRun.StartedAt)
				}

				workflowJobRun.QueueDuration = queueTime
				workflowJobRun.RunDuration = executionTime
				runs = append(runs, workflowJobRun)
			}
		}
	}

	return user.Installations, repositories, runs
}

func DestroyUserData(user *User) error {
	result := db.DB.Delete(&user) // we cascade delete all related data
	if result.Error != nil {
		return errors.New("failed to destroy user data")
	}

	return nil
}

func UpsertUser(id int64, name string, email string) (*gorm.DB, User) {
	u := User{Id: id, Name: name, Email: email}
	result := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email",
			"name",
		}),
	}).Create(&u)

	return result, u
}

func CreateWorkflowJobRun(id int64,
	name string,
	url string,
	runId int64,
	workflowName string,
	status string,
	repositoryId int64,
	startedAt time.Time) *gorm.DB {

	jobRun := &WorkflowJobRun{
		Id:            id,
		Name:          name,
		Url:           url,
		WorkflowRunId: runId,
		WorkflowName:  workflowName,
		Status:        status,
		RepositoryId:  repositoryId,
		StartedAt:     startedAt,
	}

	return db.DB.Create(&jobRun)
}

func KickoffWorkflowJobRun(id int64, repositoryId int64) *gorm.DB {
	updates := &WorkflowJobRun{KickoffAt: sql.NullTime{Time: time.Now(), Valid: true}}
	return db.DB.
		Model(&WorkflowJobRun{}).
		Where("id = ? AND repository_id = ?", id, repositoryId).
		Updates(updates)
}

func ProcessWorkflowJobRun(id int64, repositoryId int64, status string) *gorm.DB {
	updates := &WorkflowJobRun{ProcessingAt: sql.NullTime{Time: time.Now(), Valid: true}, Status: status}
	return db.DB.
		Model(&WorkflowJobRun{}).
		Where("id = ? AND repository_id = ?", id, repositoryId).
		Updates(updates)
}

func CompleteWorkflowJobRun(id int64, repositoryId int64, status string, conclusion string, endedAt time.Time) *gorm.DB {
	var c sql.NullString

	if len(conclusion) > 0 {
		c = sql.NullString{String: conclusion, Valid: true}
	} else {
		c = sql.NullString{Valid: false}
	}

	updates := &WorkflowJobRun{Status: status, Conclusion: c, EndedAt: sql.NullTime{Time: endedAt, Valid: true}}
	return db.DB.
		Model(&WorkflowJobRun{}).
		Where("id = ? AND repository_id = ?", id, repositoryId).
		Updates(updates)
}

type VMLock struct {
	Lock *gorm.DB
	VM   *VM
}

func CreateVM(baseVMName string, runnerLabel string) *gorm.DB {
	vm := VM{Status: VMAvailable, GithubRunnerLabel: runnerLabel, BaseVMName: baseVMName}
	return db.DB.Create(&vm)
}

func InaugurateVM() (*VMLock, error) {
	vmLock := VMLock{Lock: db.DB.Begin(), VM: &VM{}}
	defer func() {
		if r := recover(); r != nil {
			vmLock.Close()
		}
	}()

	result := vmLock.Start()
	if result.Error != nil {
		vmLock.Close()
		return nil, result.Error
	}

	return &vmLock, nil
}

func FreeVM(vm *VM) *gorm.DB {
	updates := map[string]interface{}{
		"external_run_id":  gorm.Expr("NULL"),
		"repository_id":    gorm.Expr("NULL"),
		"vm_instance_name": gorm.Expr("NULL"),
		"status":           VMAvailable,
	}

	return db.DB.Model(vm).Updates(updates)
}

func (vmLock *VMLock) Assign(instanceName string) *gorm.DB {
	return vmLock.Lock.Model(&vmLock.VM).Update("vm_instance_name", instanceName)
}

func (vmLock *VMLock) Commit(runId int64, repositoryInternalId int64) {
	updates := VM{
		Status:        VMProcessing,
		ExternalRunId: sql.NullInt64{Int64: runId, Valid: true},
		RepositoryId:  sql.NullInt64{Int64: repositoryInternalId, Valid: true},
	}

	vmLock.Lock.Model(&vmLock.VM).Updates(updates)
	vmLock.Lock.Commit()
}

func (vmLock *VMLock) Close() {
	vmLock.Lock.Rollback()
}

func (vmLock *VMLock) Start() *gorm.DB {
	return db.DB.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ?", VMAvailable).First(&vmLock.VM)
}
