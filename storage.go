package crond

import "github.com/oklog/ulid/v2"

type Repository struct {
	JobRepository
	CronRepository
}

type JobRepository interface {
	CreateJob(j Job) error
	UpdateJob(j Job) error
	GetJob(id ulid.ULID) (Job, error)
}

type CronRepository interface {
	CreateCronjob(j Cronjob) error
	UpdateCronjob(j Cronjob) error
	GetCronjob(id ulid.ULID) (Cronjob, error)
}
