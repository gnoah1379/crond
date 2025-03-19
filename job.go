package crond

import (
	"container/heap"
	"context"
	"github.com/oklog/ulid/v2"
	"math"
	"sync"
	"time"
)

type TaskState string

const (
	Pending   TaskState = "PENDING"
	Scheduled TaskState = "SCHEDULED"
	Running   TaskState = "RUNNING"
	Retry     TaskState = "RETRY"
	Success   TaskState = "SUCCESS"
	Failed    TaskState = "FAILED"
)

type Job struct {
	ID           ulid.ULID  `json:"id"`
	Func         string     `json:"func"`
	ScheduleTime time.Time  `json:"scheduleTime"`
	StartTime    time.Time  `json:"startTime"`
	FinishTime   time.Time  `json:"finishTime"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	Retry        int        `json:"retry"`
	Args         []any      `json:"args"`
	Result       any        `json:"result"`
	Error        any        `json:"errors"`
	Status       TaskState  `json:"status"`
	CronJobID    *ulid.ULID `json:"cronJobId"`
}

type jobQueueItem struct {
	*Job
	index int
}

type jobQueue []*jobQueueItem

func (jq jobQueue) Len() int {
	return len(jq)
}

func (jq jobQueue) Less(i, j int) bool {
	return jq[i].ScheduleTime.Before(jq[j].ScheduleTime)
}

func (jq jobQueue) Swap(i, j int) {
	jq[i], jq[j] = jq[j], jq[i]
}

func (jq *jobQueue) Push(x any) {
	*jq = append(*jq, x.(*jobQueueItem))
}

func (jq *jobQueue) Pop() any {
	old := *jq
	n := len(old)
	x := old[n-1]
	*jq = old[0 : n-1]
	return x
}

func (jq *jobQueue) Peek() *Job {
	if jq.Len() == 0 {
		return nil
	}
	return (*jq)[0].Job
}

func (jq *jobQueue) update(index int, job *Job) {
	(*jq)[index].Job = job
	heap.Fix(jq, index)
}

type JobScheduler struct {
	heap     jobQueue
	indexMap map[ulid.ULID]int
	timer    *time.Timer
	jobChan  chan *Job
	mu       sync.Mutex
}

func NewJobScheduler() *JobScheduler {
	return &JobScheduler{
		heap:     make(jobQueue, 0),
		indexMap: make(map[ulid.ULID]int),
		jobChan:  make(chan *Job),
		timer:    time.NewTimer(math.MaxInt64),
	}
}

func (js *JobScheduler) ScheduleJob(job *Job) {
	js.mu.Lock()
	defer js.mu.Unlock()
	index, ok := js.indexMap[job.ID]
	if ok {
		js.heap.update(index, job)
	} else {
		item := &jobQueueItem{Job: job}
		heap.Push(&js.heap, item)
		js.indexMap[job.ID] = item.index
	}

	index = js.indexMap[job.ID]
	if index == 0 {
		js.timer.Reset(job.ScheduleTime.Sub(time.Now()))
	}
}

func (js *JobScheduler) RemoveJob(id ulid.ULID) {
	js.mu.Lock()
	defer js.mu.Unlock()
	index, ok := js.indexMap[id]
	if !ok {
		return
	}
	heap.Remove(&js.heap, index)
	delete(js.indexMap, id)
}

func (js *JobScheduler) Run(ctx context.Context) {
	defer js.timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-js.timer.C:
			js.mu.Lock()
			job := heap.Pop(&js.heap).(*jobQueueItem).Job
			delete(js.indexMap, job.ID)
			nextJob := js.heap.Peek()
			if nextJob != nil {
				js.timer.Reset(nextJob.ScheduleTime.Sub(time.Now()))
			} else {
				js.timer.Reset(math.MaxInt64)
			}
			js.mu.Unlock()
			js.jobChan <- job
		}
	}
}

func (js *JobScheduler) NextJob() <-chan *Job {
	return js.jobChan
}
