package core

import (
	"buildkansen/config"
	"buildkansen/internal/app_error"
	"buildkansen/models"
	"fmt"
	"net/http"
)

func ValidateWorkflow(organizationId int64, repositoryId int64) (*models.Installation, *models.Repository, *app_error.AppError) {
	i, err := models.FindEntity(models.Installation{}, organizationId, "account_id")
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

func FindValidRunnerName(runnerLabels []string) (string, *app_error.AppError) {
	for _, label := range runnerLabels {
		for _, runnerName := range config.C.ValidRunnerNames {
			if label == runnerName {
				return label, nil
			}
		}
	}

	return "", app_error.NewAppError(http.StatusNotFound, "No valid runner name found", nil)
}

func CompleteWorkflow(runId int64, repositoryInternalId int64) *app_error.AppError {
	result := models.FreeVM(runId, repositoryInternalId)
	if result.Error != nil {
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to free the VM", result.Error)
	}

	return nil
}
