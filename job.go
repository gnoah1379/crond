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
	Succeeded TaskState = "SUCCEEDED"
	Failed    TaskState = "FAILED"
	Cancelled TaskState = "CANCELLED"
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

type jobHeap struct {
	jobs     []*Job
	indexMap map[ulid.ULID]int
}

func (h *jobHeap) Len() int {
	return len(h.jobs)
}

func (h *jobHeap) Less(i, j int) bool {
	return h.jobs[i].ScheduleTime.Before(h.jobs[j].ScheduleTime)
}

func (h *jobHeap) Swap(i, j int) {
	h.jobs[i], h.jobs[j] = h.jobs[j], h.jobs[i]
	h.indexMap[h.jobs[i].ID] = i
	h.indexMap[h.jobs[j].ID] = j
}

func (h *jobHeap) Push(x interface{}) {
	t := x.(*Job)
	h.jobs = append(h.jobs, t)
	h.indexMap[t.ID] = len(h.jobs) - 1
}

func (h *jobHeap) Pop() interface{} {
	n := len(h.jobs)
	t := h.jobs[n-1]
	h.jobs = h.jobs[:n-1]
	delete(h.indexMap, t.ID)
	return t
}

func (h *jobHeap) Peek() *Job {
	if len(h.jobs) == 0 {
		return nil
	}
	return h.jobs[0]
}

type JobScheduler struct {
	heap  *jobHeap
	timer *time.Timer
	repo  Repository
	mu    sync.Mutex
}

func NewJobScheduler() *JobScheduler {
	return &JobScheduler{
		heap:  &jobHeap{indexMap: make(map[ulid.ULID]int)},
		timer: time.NewTimer(math.MaxInt64),
	}
}

func (s *JobScheduler) Schedule(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, ok := s.heap.indexMap[job.ID]
	if ok {
		s.heap.jobs[index] = job
		heap.Fix(s.heap, index)
	} else {
		heap.Push(s.heap, job)
	}
	index = s.heap.indexMap[job.ID]
	if index == 0 {
		s.timer.Reset(job.ScheduleTime.Sub(time.Now()))
	}
}

func (s *JobScheduler) Remove(id ulid.ULID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, ok := s.heap.indexMap[id]
	if !ok {
		return
	}
	heap.Remove(s.heap, index)
}

func (s *JobScheduler) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.timer.C:
			s.mu.Lock()
			job := heap.Pop(s.heap).(*Job)
			s.resetTimer()
			s.mu.Unlock()
			_ = job
			// TODO save and send job to worker
		}
	}
}

func (s *JobScheduler) resetTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.heap.Peek()
	if job == nil {
		return
	}
	dur := job.ScheduleTime.Sub(time.Now())
	if dur < 0 {
		dur = 0
	}
	s.timer.Reset(dur)
}
