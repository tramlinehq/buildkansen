package jobs

import (
	"buildkansen/models"
	"fmt"
	"sync"
	"time"
)

const workerWaitTimeNs = time.Second * 5

type jobManager struct {
	jobQueue chan Job
	wg       sync.WaitGroup
}

var jobQueueManager *jobManager

func Start() {
	jobQueueManager = &jobManager{
		jobQueue: make(chan Job),
	}
	jobQueueManager.startWorkers(1)
}

func (jm *jobManager) startWorkers(numWorkers int) {
	for i := 1; i <= numWorkers; i++ {
		jm.wg.Add(1)
		go jm.worker(i)
	}
}

func (jm *jobManager) enqueueJob(job Job) {
	jm.jobQueue <- job
}

// worker infinitely processes jobs from the job queue
func (jm *jobManager) worker(id int) {
	defer jm.wg.Done()

	for {
		vmLock, err := models.InaugurateVM()
		if err != nil {
			fmt.Printf("No available VMs")
			time.Sleep(workerWaitTimeNs)
			continue
		}

		select {
		case job, ok := <-jm.jobQueue:
			if ok {
				err := job.Execute()
				if err != nil {
					fmt.Printf("Worker %d could not process job: %+v\n", id, job)
					vmLock.Close()
					continue
				}

				vmLock.Commit(job.WorkflowRunId, job.RepositoryId)
			}

			fmt.Printf("Worker %d processed job: %+v\n", id, job)
			continue
		case <-time.After(workerWaitTimeNs):
			vmLock.Close()
			fmt.Printf("No job found for %d nanoseconds, trying again\n", workerWaitTimeNs)
		}
	}
}
