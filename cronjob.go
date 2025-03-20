package crond

import (
	"container/heap"
	"context"
	"github.com/oklog/ulid/v2"
	"math"
	"sync"
	"time"
)

type Cronjob struct {
	ID               ulid.ULID     `json:"id"`
	Name             string        `json:"name"`
	Schedule         string        `json:"schedule"`
	Interval         time.Duration `json:"interval"`
	Enabled          bool          `json:"enabled"`
	Func             string        `json:"func"`
	Args             []any         `json:"args"`
	StartAt          time.Time     `json:"startAt"`
	ExpiresTime      time.Duration `json:"expiresTime"`
	LastScheduleTime time.Time     `json:"lastScheduleTime"`
	NextScheduleTime time.Time     `json:"nextScheduleTime"`
}

type cronjobHeap struct {
	cronjobs []*Cronjob
	indexMap map[ulid.ULID]int
}

func (h *cronjobHeap) Len() int {
	return len(h.cronjobs)
}

func (h *cronjobHeap) Less(i, j int) bool {
	return h.cronjobs[i].NextScheduleTime.Before(h.cronjobs[j].NextScheduleTime)
}

func (h *cronjobHeap) Swap(i, j int) {
	h.cronjobs[i], h.cronjobs[j] = h.cronjobs[j], h.cronjobs[i]
	h.indexMap[h.cronjobs[i].ID] = i
	h.indexMap[h.cronjobs[j].ID] = j
}

func (h *cronjobHeap) Push(x interface{}) {
	t := x.(*Cronjob)
	h.cronjobs = append(h.cronjobs, t)
	h.indexMap[t.ID] = len(h.cronjobs) - 1
}

func (h *cronjobHeap) Pop() interface{} {
	n := len(h.cronjobs)
	t := h.cronjobs[n-1]
	h.cronjobs = h.cronjobs[:n-1]
	delete(h.indexMap, t.ID)
	return t
}

func (h *cronjobHeap) Peek() *Cronjob {
	if len(h.cronjobs) == 0 {
		return nil
	}
	return h.cronjobs[0]
}

type CronjobScheduler struct {
	heap         *cronjobHeap
	timer        *time.Timer
	jobScheduler *JobScheduler
	repo         Repository
	mu           sync.Mutex
}

func NewCronjobScheduler() *CronjobScheduler {
	return &CronjobScheduler{
		heap:  &cronjobHeap{indexMap: make(map[ulid.ULID]int)},
		timer: time.NewTimer(math.MaxInt64),
	}
}

func (s *CronjobScheduler) Schedule(cronJob *Cronjob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, ok := s.heap.indexMap[cronJob.ID]
	if ok {
		s.heap.cronjobs[index] = cronJob
		heap.Fix(s.heap, index)
	} else {
		heap.Push(s.heap, cronJob)
	}
	index = s.heap.indexMap[cronJob.ID]
	if index == 0 {
		s.timer.Reset(cronJob.NextScheduleTime.Sub(time.Now()))
	}
}

func (s *CronjobScheduler) Remove(id ulid.ULID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, ok := s.heap.indexMap[id]
	if !ok {
		return
	}
	heap.Remove(s.heap, index)
	if index == 0 {
		cronjob := s.heap.Peek()
		if cronjob == nil {
			return
		}
		s.timer.Reset(cronjob.NextScheduleTime.Sub(time.Now()))
	}
}

func (s *CronjobScheduler) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.timer.C:
			var cronjob *Cronjob
			var job *Job
			s.mu.Lock()
			cronjob = s.heap.Peek()
			if cronjob == nil {
				continue
			}
			now := time.Now()
			job = &Job{
				ID:           ulid.Make(),
				Func:         cronjob.Func,
				ScheduleTime: now,
				ExpiresAt:    now.Add(cronjob.ExpiresTime),
				Args:         cronjob.Args,
				Status:       Pending,
				CronJobID:    &cronjob.ID,
			}

			s.jobScheduler.Schedule(job)
			cronjob.LastScheduleTime = now
			cronjob.NextScheduleTime = cronjob.NextScheduleTime.Add(cronjob.Interval)
			heap.Fix(s.heap, 0)
			s.resetTimer()
			s.mu.Unlock()
			// TODO: save job and update cronjob
			//if err := s.repo.CreateJob(job);err != nil {
			//	slog.Error("create job failed", "err", err, "cronjob", cronjob.Name)
			//	continue
			//}
			//if err := s.repo.UpdateCronjob(cronjob); err != nil {
			//	slog.Error("update cronjob failed", "cronjob", cronjob.Name, "err", err)
			//	continue
			//}
		}
	}
}

func (s *CronjobScheduler) resetTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cronjob := s.heap.Peek()
	if cronjob == nil {
		return
	}
	dur := cronjob.NextScheduleTime.Sub(time.Now())
	if dur < 0 {
		dur = 0
	}
	s.timer.Reset(dur)
}
