package core

import (
	"buildkansen/config"
	"buildkansen/internal/app_error"
	"buildkansen/models"
	"fmt"
	"net/http"
)

func ValidateWorkflow(organizationId int64, repositoryId int64) *app_error.AppError {
	_, err := models.FindEntity(models.Installation{}, organizationId, "account_id")
	if err != nil {
		fmt.Println("could not find an installation for this webhook")
		return app_error.NewAppError(http.StatusNotFound, "Failed to find an installation for this webhook", err)
	}

	_, err = models.FindEntityById(models.Repository{}, repositoryId)
	if err != nil {
		fmt.Println("could not find a repository for this webhook")
		return app_error.NewAppError(http.StatusNotFound, "Failed to find a repository for this webhook", err)
	}

	return nil
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

func CompleteWorkflow(runId int64, repositoryId int64) *app_error.AppError {
	result := models.FreeVM(runId, repositoryId)
	if result.Error != nil {
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to free the VM", result.Error)
	}

	return nil
}
