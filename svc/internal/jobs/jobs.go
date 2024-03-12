package jobs

import (
	"buildkansen/config"
	githubApi "buildkansen/github"
	"buildkansen/models"
	"fmt"
	"os/exec"
	"time"

	"github.com/google/uuid"
)

const (
	kickOffScript    = "./guest.vm.up"
	kickOffScriptDir = "../host"
)

type Job struct {
	AccountLogin          string
	RepositoryInternalId  int64
	RepositoryUrl         string
	InstallationId        int64
	RunnerName            string
	WorkflowRunId         int64
	WorkflowRunName       string
	WorkflowRunStatus     string
	WorkflowRunConclusion string
	WorkflowJobId         int64
	WorkflowJobName       string
	WorkflowJobUrl        string
	WorkflowJobStart      time.Time
}

func NewJob(accountLogin string,
	repositoryInternalId int64, repositoryUrl string,
	installationId int64,
	runnerName string,
	runId int64, runName string, runStatus string, runConclusion string,
	jobId int64, jobName string, jobUrl string, jobStart time.Time) *Job {

	return &Job{
		AccountLogin:          accountLogin,
		RepositoryInternalId:  repositoryInternalId,
		RepositoryUrl:         repositoryUrl,
		InstallationId:        installationId,
		RunnerName:            runnerName,
		WorkflowRunId:         runId,
		WorkflowRunName:       runName,
		WorkflowRunStatus:     runStatus,
		WorkflowRunConclusion: runConclusion,
		WorkflowJobId:         jobId,
		WorkflowJobName:       jobName,
		WorkflowJobUrl:        jobUrl,
		WorkflowJobStart:      jobStart,
	}
}

func (job *Job) Enqueue() {
	go job.createWorkflowJobRun()
	fmt.Printf("enqueuing job: %d\n", job.WorkflowJobId)
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
	go job.kickoffWorkflowJobRun()

	return nil
}

func (job *Job) createWorkflowJobRun() {
	result := models.CreateWorkflowJobRun(job.WorkflowJobId,
		job.WorkflowJobName,
		job.WorkflowJobUrl,
		job.WorkflowRunId,
		job.WorkflowRunName,
		job.WorkflowRunStatus,
		job.RepositoryInternalId,
		job.WorkflowJobStart)

	if result.Error != nil {
		fmt.Println("could not create a workflow job run: ", result.Error)
	}
}

func (job *Job) kickoffWorkflowJobRun() {
	result := models.KickoffWorkflowJobRun(job.WorkflowJobId, job.RepositoryInternalId)

	if result.Error != nil {
		fmt.Println("could not mark workflow job run started: ", result.Error)
	}
}
