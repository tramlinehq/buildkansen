package core

import (
	"buildkansen/config"
	"buildkansen/db"
	githubApi "buildkansen/github"
	"buildkansen/internal/app_error"
	"buildkansen/models"
	"net/http"
)

func CreateOrUpdateUser(id int64, name string, email string) *app_error.AppError {
	result := models.UpsertUser(id, name, email)
	if result.Error != nil {
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to create/update the user", result.Error)
	}
	return nil
}

func CreateInstallation(userId int64, installationId int64) *app_error.AppError {
	client, err := githubApi.NewClient(config.C.GithubAppId, installationId, config.C.GithubPrivateKeyBase64)

	if err != nil {
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to create a GitHub client", err)
	}

	githubInstallation, _, _ := client.GetInstallation()
	githubRepositories, _, _ := client.GetInstallationRepos()

	tx := db.DB.Begin()

	installation := models.Installation{
		Id:               *githubInstallation.ID,
		AccountType:      *githubInstallation.Account.Type,
		AccountID:        *githubInstallation.Account.ID,
		AccountLogin:     *githubInstallation.Account.Login,
		AccountAvatarUrl: *githubInstallation.Account.AvatarURL,
		UserId:           userId,
	}
	result := tx.Create(&installation)

	if result.Error != nil {
		tx.Rollback()
		return app_error.NewAppError(http.StatusInternalServerError, "Failed to save the installation", result.Error)
	}

	for _, repo := range githubRepositories.Repositories {
		repository := models.Repository{
			Id:             *repo.ID,
			Name:           *repo.Name,
			FullName:       *repo.FullName,
			Private:        *repo.Private,
			InstallationId: installation.Id,
		}

		result := tx.Create(&repository)

		if result.Error != nil {
			tx.Rollback()
			return app_error.NewAppError(http.StatusInternalServerError, "Failed to save the repositories", result.Error)
		}
	}

	tx.Commit()

	return nil
}
