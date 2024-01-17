package jobs

import (
	"buildkansen/config"
	githubApi "buildkansen/github"
	"buildkansen/internal/app_error"
	"buildkansen/models"
	"fmt"
	"net/http"
	"os/exec"
)

const (
	kickOffScript    = "./runner.kickoff"
	kickOffScriptDir = "../host"
)

type Job struct {
	OrganizationId    int64
	OrganizationLogin string
	RepositoryId      int64
	RepositoryUrl     string
	InstallationId    int64
	RunnerName        string
}

func NewJob(organizationId int64, organizationLogin string, repositoryId int64, repositoryUrl string, installationId int64, runnerName string) *Job {
	return &Job{
		OrganizationId:    organizationId,
		OrganizationLogin: organizationLogin,
		RepositoryId:      repositoryId,
		RepositoryUrl:     repositoryUrl,
		InstallationId:    installationId,
		RunnerName:        runnerName,
	}
}

func (job *Job) Process() *app_error.AppError {
	_, err := models.FindEntity(models.Installation{}, job.OrganizationId, "account_id")
	if err != nil {
		fmt.Println("could not find an installation for this webhook")
		return app_error.NewAppError(http.StatusNotFound, "Failed to find an installation for this webhook", err)
	}

	_, err = models.FindEntityById(models.Repository{}, job.RepositoryId)
	if err != nil {
		fmt.Println("could not find a repository for this webhook")
		return app_error.NewAppError(http.StatusNotFound, "Failed to find a repository for this webhook", err)
	}

	JobQueueManager.EnqueueJob(*job)

	return nil
}

func (job *Job) Execute() error {
	repo, err := models.FindEntityById(models.Repository{}, job.RepositoryId)
	if err != nil {
		fmt.Println("could not find a repository for this webhook")
		return err
	}

	client, err := githubApi.NewClient(config.C.GithubAppId, job.InstallationId, config.C.GithubPrivateKeyBase64)
	token, _, err := client.GetActionsRegistrationToken(job.OrganizationLogin, repo.(models.Repository).Name)
	if err != nil {
		fmt.Println("could not get registration token: ", err.Error())
		return err
	}

	vm, err := models.FindEntity(models.VM{}, job.RunnerName, "label")

	args := []string{
		"-i", vm.(models.VM).VMIPAddress,
		"-l", vm.(models.VM).GithubRunnerLabel,
		"-t", *token.Token,
		"-r", job.RepositoryUrl,
	}
	fmt.Printf("Executing runner script with following args: %v", args)
	cmd := exec.Command(kickOffScript, args...)
	cmd.Dir = kickOffScriptDir
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println("kicked off the runner.kickoff script!")
	return nil
}
