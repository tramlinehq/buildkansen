package core

import (
	"buildkansen/config"
	"buildkansen/internal/app_error"
	"buildkansen/models"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

const (
	purgeScript    = "./guest.vm.down"
	purgeScriptDir = "../host"
)

func ValidateWorkflow(installationId int64, repositoryId int64) (*models.Installation, *models.Repository, *app_error.AppError) {
	i, err := models.FindEntityById(models.Installation{}, installationId)
	if err != nil {
		fmt.Println("could not find an installation for this webhook")
		return nil, nil, app_error.NewAppError(http.StatusNotFound, "Failed to find an installation for this webhook", err)
	}

	installation := i.(models.Installation)
	repository, err := models.FindRepositoryByInstallation(installation.InternalId, repositoryId)
	if err != nil {
		fmt.Println("could not find a repository for this webhook")
		return nil, nil, app_error.NewAppError(http.StatusNotFound, "Failed to find a repository for this webhook", err)
	}

	return &installation, repository, nil
}

func FindValidRunnerName(runnerLabels []string) (string, bool) {
	for _, label := range runnerLabels {
		for _, runnerName := range config.C.ValidRunnerNames {
			if label == runnerName {
				return label, true
			}
		}
	}

	return "", false
}

func ProcessWorkflowRun(jobId int64, runStatus string, repoId int64) {
	fmt.Printf("updating workflow job run for: %d with status: %s\n", jobId, runStatus)
	result := models.ProcessWorkflowJobRun(jobId, repoId, runStatus)
	if result.Error != nil {
		fmt.Printf("could not update workflow job for : %d", jobId)
	}
}

func CompleteWorkflow(jobId int64, runId int64, runStatus string, runConclusion string, repoId int64, endedAt time.Time) *app_error.AppError {
	vm, err := models.FindEntity(models.VM{}, runId, "external_run_id")
	if err != nil {
		fmt.Println("Error:", err)
		return app_error.NewAppError(http.StatusNotFound, "No valid runner was found", err)
	}

	go func() {
		fmt.Printf("updating workflow job run for: %d with conclusion: %s, and status: %s\n", jobId, runConclusion, runStatus)
		result := models.CompleteWorkflowJobRun(jobId, repoId, runStatus, runConclusion, endedAt)
		if result.Error != nil {
			fmt.Printf("could not update workflow job for : %d", jobId)
		}
	}()

	vmModel := vm.(models.VM)
	args := []string{
		"-n", vmModel.VMInstanceName,
	}
	fmt.Printf("Executing purge script with following args: %v", args)
	cmd := exec.Command(purgeScript, args...)
	cmd.Dir = purgeScriptDir
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to purge the VM", err)
	}

	result := models.FreeVM(&vmModel)
	if result.Error != nil {
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to free the VM", result.Error)
	}

	return nil
}
