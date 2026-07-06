package server

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Job struct {
	ID      int               `json:"id"`
	Status  string            `json:"status"`
	Created time.Time         `json:"created"`
	Config  generationRequest `json:"config"`
	Tables  int               `json:"tables"`
	Errors  []string          `json:"errors,omitempty"`
	Data    []byte            `json:"-"`
	Format  string            `json:"format"`
}

type jobSummary struct {
	ID      int       `json:"id"`
	Status  string    `json:"status"`
	Created time.Time `json:"created"`
	Tables  int       `json:"tables"`
	Format  string    `json:"format"`
	Rows    int       `json:"rows"`
	Errors  []string  `json:"errors,omitempty"`
}

type jobDetail struct {
	ID      int               `json:"id"`
	Status  string            `json:"status"`
	Created time.Time         `json:"created"`
	Config  generationRequest `json:"config"`
	Tables  int               `json:"tables"`
	Errors  []string          `json:"errors,omitempty"`
	Data    string            `json:"data"`
	Format  string            `json:"format"`
}

type JobStore struct {
	mu         sync.Mutex
	jobs       []*Job
	nextID     int
	persistPath string
}

func NewJobStore() *JobStore {
	return &JobStore{
		jobs:   make([]*Job, 0),
		nextID: 1,
	}
}

func NewJobStoreWithPersistence(persistPath string) *JobStore {
	jobStore := &JobStore{
		jobs:        make([]*Job, 0),
		nextID:      1,
		persistPath: persistPath,
	}
	jobStore.loadFromDisk()
	return jobStore
}

func (jobStore *JobStore) Add(job *Job) {
	jobStore.mu.Lock()
	defer jobStore.mu.Unlock()
	job.ID = jobStore.nextID
	jobStore.nextID++
	jobStore.jobs = append(jobStore.jobs, job)
	if jobStore.persistPath != "" {
		jobStore.appendToDisk(job)
	}
}

func (jobStore *JobStore) List() []jobSummary {
	jobStore.mu.Lock()
	defer jobStore.mu.Unlock()

	summaries := make([]jobSummary, len(jobStore.jobs))
	for index, job := range jobStore.jobs {
		summaries[index] = jobSummary{
			ID:      job.ID,
			Status:  job.Status,
			Created: job.Created,
			Tables:  job.Tables,
			Format:  job.Format,
			Rows:    job.Config.Rows,
			Errors:  job.Errors,
		}
	}
	reverseSlice(summaries)
	return summaries
}

func (jobStore *JobStore) GetByID(id int) *Job {
	jobStore.mu.Lock()
	defer jobStore.mu.Unlock()
	for _, job := range jobStore.jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

func (jobStore *JobStore) Delete(id int) bool {
	jobStore.mu.Lock()
	defer jobStore.mu.Unlock()
	for index, job := range jobStore.jobs {
		if job.ID == id {
			jobStore.jobs = append(jobStore.jobs[:index], jobStore.jobs[index+1:]...)
			if jobStore.persistPath != "" {
				jobStore.rewriteDisk()
			}
			return true
		}
	}
	return false
}

func (jobStore *JobStore) rewriteDisk() {
	file, openError := os.Create(jobStore.persistPath)
	if openError != nil {
		return
	}
	defer file.Close()
	json.NewEncoder(file).Encode(jobStore.jobs)
}

func reverseSlice[T any](slice []T) {
	for left, right := 0, len(slice)-1; left < right; left, right = left+1, right-1 {
		slice[left], slice[right] = slice[right], slice[left]
	}
}

func (jobStore *JobStore) loadFromDisk() {
	data, readError := os.ReadFile(jobStore.persistPath)
	if readError != nil {
		return
	}
	var loadedJobs []*Job
	if unmarshalError := json.Unmarshal(data, &loadedJobs); unmarshalError != nil {
		return
	}
	jobStore.jobs = loadedJobs
	for _, j := range loadedJobs {
		if j.ID >= jobStore.nextID {
			jobStore.nextID = j.ID + 1
		}
	}
}

func (jobStore *JobStore) appendToDisk(job *Job) {
	file, openError := os.OpenFile(jobStore.persistPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openError != nil {
		return
	}
	defer file.Close()
	json.NewEncoder(file).Encode(job)
}
