package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Job is the interface all scheduled jobs must implement.
type Job interface {
	// Name returns a human-readable identifier for the job.
	Name() string
	// Run executes one iteration of the job.
	Run(ctx context.Context) error
	// Interval returns how often the job should run.
	Interval() time.Duration
}

// Scheduler manages a set of background jobs, each running in its own
// goroutine on a ticker.
type Scheduler struct {
	jobs   []Job
	logger zerolog.Logger
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New creates a new Scheduler.
func New(logger zerolog.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
	}
}

// Register adds a job to the scheduler. Must be called before Start.
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
}

// Start launches all registered jobs in background goroutines. Each job
// runs immediately once and then on its configured interval. When the
// context is cancelled, jobs finish their current iteration and stop.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.runJob(ctx, job)
	}

	s.logger.Info().Int("jobs", len(s.jobs)).Msg("scheduler started")
}

// Stop cancels the context and waits for all jobs to finish.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.logger.Info().Msg("scheduler stopped")
}

// runJob executes a single job on a ticker loop. It runs the job
// immediately, then again on each tick until the context is cancelled.
func (s *Scheduler) runJob(ctx context.Context, job Job) {
	defer s.wg.Done()

	s.logger.Info().
		Str("job", job.Name()).
		Dur("interval", job.Interval()).
		Msg("starting scheduled job")

	// Run immediately on startup.
	s.executeJob(ctx, job)

	ticker := time.NewTicker(job.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Str("job", job.Name()).Msg("job stopped")
			return
		case <-ticker.C:
			s.executeJob(ctx, job)
		}
	}
}

// executeJob runs a single iteration of a job, logging any errors.
func (s *Scheduler) executeJob(ctx context.Context, job Job) {
	if err := job.Run(ctx); err != nil {
		s.logger.Error().
			Err(err).
			Str("job", job.Name()).
			Msg("job execution failed")
	}
}
