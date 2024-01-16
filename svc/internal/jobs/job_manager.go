package jobs

import (
	"fmt"
	"sync"
)

type JobManager struct {
	jobQueue chan Job
	wg       sync.WaitGroup
}

var JobQueueManager *JobManager

func Init() {
	JobQueueManager = &JobManager{
		jobQueue: make(chan Job),
	}
	// Start workers
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

	for job := range jm.jobQueue {
		job.Execute()

		fmt.Printf("Worker %d processed job: %+v\n", id, job)
	}
}
