package jobs

import (
	"buildkansen/config"
	githubApi "buildkansen/github"
	"buildkansen/models"
	"fmt"
	"os/exec"

	"github.com/google/uuid"
)

const (
	kickOffScript    = "./guest.vm.up"
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

func (job *Job) Execute(vmLock *models.VMLock) error {
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

	newUUID := uuid.New()
	uuidString := newUUID.String()
	runnerName := vmLock.VM.BaseVMName + "-" + uuidString
	result := vmLock.Assign(runnerName)
	if result.Error != nil {
		fmt.Printf("could not update runner for VM: %s", runnerName)
		return result.Error
	}

	args := []string{
		"-b", vmLock.VM.BaseVMName,
		"-n", runnerName,
		"-l", vmLock.VM.GithubRunnerLabel,
		"-t", *token.Token,
		"-r", job.RepositoryUrl,
	}
	fmt.Printf("executing runner script with following args: %v", args)
	cmd := exec.Command(kickOffScript, args...)
	cmd.Dir = kickOffScriptDir
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	fmt.Printf("kicked off the %s script!", kickOffScript)

	return nil
}
