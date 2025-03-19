package crond

import (
	"github.com/oklog/ulid/v2"
	"time"
)

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

func (cj CronJob) GetID() ulid.ULID {
	return cj.ID
}

func (cj CronJob) GetScheduleTime() time.Time {
	return cj.NextScheduleTime
}
