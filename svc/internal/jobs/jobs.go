package jobs

import (
	"buildkansen/config"
	githubApi "buildkansen/github"
	"buildkansen/models"
	"fmt"
	"os/exec"
)

const (
	kickOffScript    = "./runner.kickoff"
	kickOffScriptDir = "../host"
)

type Job struct {
	AccountLogin         string
	RepositoryInternalId int64
	RepositoryUrl        string
	InstallationId       int64
	RunnerName           string
	WorkflowRunId        int64
}

func NewJob(accountLogin string, repositoryInternalId int64, repositoryUrl string, installationId int64, runnerName string, runId int64) *Job {
	return &Job{
		AccountLogin:         accountLogin,
		RepositoryInternalId: repositoryInternalId,
		RepositoryUrl:        repositoryUrl,
		InstallationId:       installationId,
		RunnerName:           runnerName,
		WorkflowRunId:        runId,
	}
}

func (job *Job) Process() {
	jobQueueManager.enqueueJob(*job)
}

func (job *Job) Execute() error {
	repo, err := models.FindEntity(models.Repository{}, job.RepositoryInternalId, "internal_id")
	if err != nil {
		fmt.Println("could not find a repository for this webhook")
		return err
	}

	client, err := githubApi.NewClient(config.C.GithubAppId, job.InstallationId, config.C.GithubPrivateKeyBase64)
	token, _, err := client.GetActionsRegistrationToken(job.AccountLogin, repo.(models.Repository).Name)
	if err != nil {
		fmt.Println("could not get registration token: ", err.Error())
		return err
	}

	vm, err := models.FindEntity(models.VM{}, job.RunnerName, "github_runner_label")

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
