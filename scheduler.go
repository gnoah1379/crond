package crond

import (
	"github.com/oklog/ulid/v2"
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
	ID           ulid.ULID `json:"id"`
	Func         string    `json:"func"`
	WorkerAssign string    `json:"workerAssign"`
	ScheduleTime time.Time `json:"scheduleTime"`
	StartTime    time.Time `json:"startTime"`
	FinishTime   time.Time `json:"finishTime"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Retry        int       `json:"retry"`
	Args         []any     `json:"args"`
	Result       any       `json:"result"`
	Error        any       `json:"errors"`
	Status       TaskState `json:"status"`
}

func (j Job) Compare(j2 Job) int {
	val := j.ScheduleTime.Compare(j2.ScheduleTime)
	if val == 0 {
		return j.ID.Compare(j2.ID)
	}
	return val
}

type CronJob struct {
	ID               ulid.ULID `json:"id"`
	Name             string    `json:"name"`
	Schedule         string    `json:"schedule"`
	Enabled          bool      `json:"enabled"`
	Func             string    `json:"func"`
	Args             []any     `json:"args"`
	StartAt          time.Time `json:"startAt"`
	ExpiresTime      time.Time `json:"expiresTime"`
	LastScheduleTime time.Time `json:"lastScheduleTime"`
	NextScheduleTime time.Time `json:"nextScheduleTime"`
}

func (cj CronJob) Compare(cj2 CronJob) int {
	val := cj.NextScheduleTime.Compare(cj2.NextScheduleTime)
	if val == 0 {
		return cj.ID.Compare(cj2.ID)
	}
	return val

}

type Scheduler struct {
}
