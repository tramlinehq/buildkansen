package jobs

import (
	"buildkansen/models"
	"fmt"
	"sync"
	"time"
)

const workerWaitTimeNs = time.Second * 5

type JobManager struct {
	jobQueue chan Job
	wg       sync.WaitGroup
}

var JobQueueManager *JobManager

func Start() {
	JobQueueManager = &JobManager{
		jobQueue: make(chan Job),
	}
	JobQueueManager.StartWorkers(1)
}

// StartWorkers starts the specified number of worker goroutines
func (jm *JobManager) StartWorkers(numWorkers int) {
	for i := 1; i <= numWorkers; i++ {
		jm.wg.Add(1)
		go jm.worker(i)
	}
}

// EnqueueJob enqueues a job to the job queue
func (jm *JobManager) EnqueueJob(job Job) {
	jm.jobQueue <- job
}

// worker is the worker goroutine that processes jobs from the queue
func (jm *JobManager) worker(id int) {
	defer jm.wg.Done()

	for {
		vmLock, err := models.FindVM()
		if err != nil {
			fmt.Printf("No available VMs")
			time.Sleep(workerWaitTimeNs)
			continue
		}

		select {
		case job, ok := <-jm.jobQueue:
			if ok {
				vmLock.Commit()
				err := job.Execute()

				if err != nil {
					fmt.Printf("Worker %d could not process job: %+v\n", id, job)
				}
			}

			fmt.Printf("Worker %d processed job: %+v\n", id, job)
			continue
		case <-time.After(workerWaitTimeNs):
			vmLock.Close()
			fmt.Printf("No job found for %d nanoseconds, trying again\n", workerWaitTimeNs)
		}
	}
}
