package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// SeriesGenerator is the interface for generating occurrences for all active
// event series.
type SeriesGenerator interface {
	GenerateOccurrencesForAll(ctx context.Context) error
}

// SeriesGeneratorJob periodically generates new occurrences for active event
// series, maintaining a rolling window of upcoming events.
type SeriesGeneratorJob struct {
	seriesService SeriesGenerator
	logger        zerolog.Logger
}

// NewSeriesGeneratorJob creates a new SeriesGeneratorJob.
func NewSeriesGeneratorJob(seriesService SeriesGenerator, logger zerolog.Logger) *SeriesGeneratorJob {
	return &SeriesGeneratorJob{
		seriesService: seriesService,
		logger:        logger,
	}
}

// Name returns the job identifier.
func (j *SeriesGeneratorJob) Name() string {
	return "series_generator"
}

// Interval returns how often this job runs.
func (j *SeriesGeneratorJob) Interval() time.Duration {
	return 1 * time.Hour
}

// Run executes one iteration of the series generator job.
func (j *SeriesGeneratorJob) Run(ctx context.Context) error {
	if err := j.seriesService.GenerateOccurrencesForAll(ctx); err != nil {
		j.logger.Error().Err(err).Msg("series generator: failed")
		return err
	}
	return nil
}
