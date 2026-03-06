package event

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// SeriesService contains the business logic for recurring event series.
type SeriesService struct {
	seriesStore        *SeriesStore
	eventStore         *Store
	eventService       *Service
	defaultRetention   int
	onCreateOccurrence func(ctx context.Context, seriesID, occurrenceID string)
	logger             zerolog.Logger
}

// NewSeriesService creates a new SeriesService.
func NewSeriesService(seriesStore *SeriesStore, eventStore *Store, eventService *Service, defaultRetention int, logger zerolog.Logger) *SeriesService {
	return &SeriesService{
		seriesStore:      seriesStore,
		eventStore:       eventStore,
		eventService:     eventService,
		defaultRetention: defaultRetention,
		logger:           logger,
	}
}

// SetOnCreateOccurrence registers a callback that is invoked after a new
// occurrence is generated. This is used to copy invite cards from earlier
// occurrences.
func (s *SeriesService) SetOnCreateOccurrence(fn func(ctx context.Context, seriesID, occurrenceID string)) {
	s.onCreateOccurrence = fn
}

// CreateSeries validates the request, creates a new series, and generates
// initial occurrences.
func (s *SeriesService) CreateSeries(ctx context.Context, organizerID string, req CreateSeriesRequest) (*EventSeries, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.StartDate == "" {
		return nil, fmt.Errorf("startDate is required")
	}
	if req.EventTime == "" {
		return nil, fmt.Errorf("eventTime is required")
	}
	if !isValidRecurrenceRule(req.RecurrenceRule) {
		return nil, fmt.Errorf("invalid recurrenceRule: must be weekly, biweekly, or monthly")
	}

	if req.Timezone == "" {
		req.Timezone = "America/New_York"
	}

	retentionDays := s.defaultRetention
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		retentionDays = *req.RetentionDays
	}

	contactRequirement := "email"
	if req.ContactRequirement != nil && *req.ContactRequirement != "" {
		if !isValidContactRequirement(*req.ContactRequirement) {
			return nil, fmt.Errorf("invalid contactRequirement: must be email, phone, email_or_phone, or email_and_phone")
		}
		contactRequirement = *req.ContactRequirement
	}

	showHeadcount := false
	if req.ShowHeadcount != nil {
		showHeadcount = *req.ShowHeadcount
	}
	showGuestList := false
	if req.ShowGuestList != nil {
		showGuestList = *req.ShowGuestList
	}

	var recurrenceEnd *time.Time
	if req.RecurrenceEnd != nil && *req.RecurrenceEnd != "" {
		t, err := parseFlexibleTime(*req.RecurrenceEnd)
		if err != nil {
			return nil, fmt.Errorf("invalid recurrenceEnd format: %w", err)
		}
		recurrenceEnd = &t
	}

	if req.MaxCapacity != nil && *req.MaxCapacity < 1 {
		return nil, fmt.Errorf("maxCapacity must be at least 1")
	}

	series := &EventSeries{
		ID:                      uuid.Must(uuid.NewV7()).String(),
		OrganizerID:             organizerID,
		Title:                   req.Title,
		Description:             req.Description,
		Location:                req.Location,
		Timezone:                req.Timezone,
		EventTime:               req.EventTime,
		DurationMinutes:         req.DurationMinutes,
		RecurrenceRule:          req.RecurrenceRule,
		RecurrenceEnd:           recurrenceEnd,
		MaxOccurrences:          req.MaxOccurrences,
		SeriesStatus:            "active",
		RetentionDays:           retentionDays,
		ContactRequirement:      contactRequirement,
		ShowHeadcount:           showHeadcount,
		ShowGuestList:           showGuestList,
		RSVPDeadlineOffsetHours: req.RSVPDeadlineOffsetHours,
		MaxCapacity:             req.MaxCapacity,
	}

	if err := s.seriesStore.Create(ctx, series); err != nil {
		return nil, err
	}

	// Generate initial occurrences starting from the provided start date.
	if err := s.generateInitialOccurrences(ctx, series, req.StartDate); err != nil {
		s.logger.Error().Err(err).Str("series_id", series.ID).Msg("failed to generate initial occurrences")
	}

	return series, nil
}

// generateInitialOccurrences creates the first batch of occurrences using the
// provided startDate as the first occurrence date.
func (s *SeriesService) generateInitialOccurrences(ctx context.Context, series *EventSeries, startDate string) error {
	firstDate, err := parseFlexibleTime(startDate + "T" + series.EventTime)
	if err != nil {
		// Try parsing the startDate as-is (may already include time).
		firstDate, err = parseFlexibleTime(startDate)
		if err != nil {
			return fmt.Errorf("invalid startDate format: %w", err)
		}
	}

	needed := 4
	currentDate := firstDate
	originalDay := firstDate.Day()

	for i := 0; i < needed; i++ {
		if series.MaxOccurrences != nil && i >= *series.MaxOccurrences {
			break
		}
		if series.RecurrenceEnd != nil && currentDate.After(*series.RecurrenceEnd) {
			break
		}

		occurrence := s.buildOccurrenceFromSeries(series, currentDate, i+1)
		if err := s.eventService.CreateFromSeries(ctx, occurrence); err != nil {
			return fmt.Errorf("create initial occurrence %d: %w", i, err)
		}

		if s.onCreateOccurrence != nil {
			s.onCreateOccurrence(ctx, series.ID, occurrence.ID)
		}

		currentDate = calculateNextOccurrence(currentDate, series.RecurrenceRule, originalDay)
	}

	return nil
}

// GenerateOccurrences maintains a rolling window of 4 upcoming occurrences
// for the given series.
func (s *SeriesService) GenerateOccurrences(ctx context.Context, seriesID string) error {
	series, err := s.seriesStore.FindByID(ctx, seriesID)
	if err != nil {
		return err
	}
	if series == nil || series.SeriesStatus != "active" {
		return nil
	}

	existing, err := s.eventStore.FindFutureBySeriesID(ctx, seriesID)
	if err != nil {
		return err
	}

	needed := 4 - len(existing)
	if needed <= 0 {
		return nil
	}

	var lastDate time.Time
	if len(existing) > 0 {
		lastDate = existing[len(existing)-1].EventDate
	} else {
		// Look at the most recent occurrence to find the last date.
		allEvents, err := s.eventStore.FindBySeriesID(ctx, seriesID)
		if err != nil {
			return err
		}
		if len(allEvents) > 0 {
			lastDate = allEvents[len(allEvents)-1].EventDate
		} else {
			lastDate = time.Now().UTC()
		}
	}

	totalGenerated, err := s.eventStore.CountBySeriesID(ctx, seriesID)
	if err != nil {
		return err
	}

	// Determine the original day of month from the event time for monthly clamping.
	originalDay := lastDate.Day()

	for i := 0; i < needed; i++ {
		nextDate := calculateNextOccurrence(lastDate, series.RecurrenceRule, originalDay)

		if series.MaxOccurrences != nil && totalGenerated >= *series.MaxOccurrences {
			break
		}
		if series.RecurrenceEnd != nil && nextDate.After(*series.RecurrenceEnd) {
			break
		}

		occurrence := s.buildOccurrenceFromSeries(series, nextDate, totalGenerated+1)
		if err := s.eventService.CreateFromSeries(ctx, occurrence); err != nil {
			return fmt.Errorf("create occurrence %d: %w", i, err)
		}

		if s.onCreateOccurrence != nil {
			s.onCreateOccurrence(ctx, series.ID, occurrence.ID)
		}

		lastDate = nextDate
		totalGenerated++
	}

	return nil
}

// GenerateOccurrencesForAll generates occurrences for all active series.
// Called by the background job.
func (s *SeriesService) GenerateOccurrencesForAll(ctx context.Context) error {
	activeSeries, err := s.seriesStore.FindAllActive(ctx)
	if err != nil {
		return fmt.Errorf("find active series: %w", err)
	}

	for _, series := range activeSeries {
		if err := s.GenerateOccurrences(ctx, series.ID); err != nil {
			s.logger.Error().Err(err).Str("series_id", series.ID).Msg("failed to generate occurrences")
			// Continue with other series even if one fails.
		}
	}

	return nil
}

// UpdateSeries applies partial updates to a series and regenerates
// non-overridden future occurrences.
func (s *SeriesService) UpdateSeries(ctx context.Context, seriesID, organizerID string, req UpdateSeriesRequest) (*EventSeries, error) {
	series, err := s.seriesStore.FindByID(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if series == nil {
		return nil, fmt.Errorf("series not found")
	}
	if series.OrganizerID != organizerID {
		return nil, fmt.Errorf("forbidden: you do not own this series")
	}

	if req.Title != nil {
		series.Title = *req.Title
	}
	if req.Description != nil {
		series.Description = *req.Description
	}
	if req.Location != nil {
		series.Location = *req.Location
	}
	if req.Timezone != nil {
		series.Timezone = *req.Timezone
	}
	if req.EventTime != nil {
		series.EventTime = *req.EventTime
	}
	if req.DurationMinutes != nil {
		series.DurationMinutes = req.DurationMinutes
	}
	if req.RecurrenceEnd != nil {
		if *req.RecurrenceEnd == "" {
			series.RecurrenceEnd = nil
		} else {
			t, err := parseFlexibleTime(*req.RecurrenceEnd)
			if err != nil {
				return nil, fmt.Errorf("invalid recurrenceEnd format: %w", err)
			}
			series.RecurrenceEnd = &t
		}
	}
	if req.MaxOccurrences != nil {
		series.MaxOccurrences = req.MaxOccurrences
	}
	if req.RetentionDays != nil {
		series.RetentionDays = *req.RetentionDays
	}
	if req.ContactRequirement != nil {
		if !isValidContactRequirement(*req.ContactRequirement) {
			return nil, fmt.Errorf("invalid contactRequirement: must be email, phone, email_or_phone, or email_and_phone")
		}
		series.ContactRequirement = *req.ContactRequirement
	}
	if req.ShowHeadcount != nil {
		series.ShowHeadcount = *req.ShowHeadcount
	}
	if req.ShowGuestList != nil {
		series.ShowGuestList = *req.ShowGuestList
	}
	if req.RSVPDeadlineOffsetHours != nil {
		series.RSVPDeadlineOffsetHours = req.RSVPDeadlineOffsetHours
	}
	if req.MaxCapacity != nil {
		if *req.MaxCapacity == 0 {
			series.MaxCapacity = nil
		} else if *req.MaxCapacity < 0 {
			return nil, fmt.Errorf("maxCapacity must be a positive number, or 0 to remove the limit")
		} else {
			series.MaxCapacity = req.MaxCapacity
		}
	}

	if err := s.seriesStore.Update(ctx, series); err != nil {
		return nil, err
	}

	// Update non-overridden future occurrences to match the new series template.
	if err := s.updateFutureOccurrences(ctx, series); err != nil {
		s.logger.Error().Err(err).Str("series_id", seriesID).Msg("failed to update future occurrences")
	}

	return series, nil
}

// updateFutureOccurrences updates non-overridden future events to match
// the current series template.
func (s *SeriesService) updateFutureOccurrences(ctx context.Context, series *EventSeries) error {
	futureEvents, err := s.eventStore.FindFutureBySeriesID(ctx, series.ID)
	if err != nil {
		return err
	}

	for _, ev := range futureEvents {
		if ev.SeriesOverride {
			continue
		}

		ev.Title = series.Title
		ev.Description = series.Description
		ev.Location = series.Location
		ev.Timezone = series.Timezone
		ev.RetentionDays = series.RetentionDays
		ev.ContactRequirement = series.ContactRequirement
		ev.ShowHeadcount = series.ShowHeadcount
		ev.ShowGuestList = series.ShowGuestList
		ev.MaxCapacity = series.MaxCapacity
		ev.WaitlistEnabled = series.MaxCapacity != nil

		if err := s.eventStore.Update(ctx, ev); err != nil {
			s.logger.Error().Err(err).Str("event_id", ev.ID).Msg("failed to update series occurrence")
		}
	}

	return nil
}

// StopSeries sets the series status to stopped and cancels all future
// unpublished occurrences.
func (s *SeriesService) StopSeries(ctx context.Context, seriesID, organizerID string) (*EventSeries, error) {
	series, err := s.seriesStore.FindByID(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if series == nil {
		return nil, fmt.Errorf("series not found")
	}
	if series.OrganizerID != organizerID {
		return nil, fmt.Errorf("forbidden: you do not own this series")
	}

	series.SeriesStatus = "stopped"
	if err := s.seriesStore.Update(ctx, series); err != nil {
		return nil, err
	}

	// Cancel future events that haven't been individually overridden.
	// Overridden occurrences are left as-is since the organizer customized them.
	futureEvents, err := s.eventStore.FindFutureBySeriesID(ctx, seriesID)
	if err != nil {
		s.logger.Error().Err(err).Str("series_id", seriesID).Msg("failed to find future events for stop")
		return series, nil
	}

	for _, ev := range futureEvents {
		if ev.SeriesOverride {
			continue // Don't cancel individually modified occurrences
		}
		// Use the event service Cancel for published events so the onCancel
		// callback fires and attendees receive cancellation notifications.
		if ev.Status == "published" {
			if _, err := s.eventService.Cancel(ctx, ev.ID, series.OrganizerID, true); err != nil {
				s.logger.Error().Err(err).Str("event_id", ev.ID).Msg("failed to cancel published series occurrence")
			}
			continue
		}
		// For non-published events (draft), cancel directly via the store.
		ev.Status = "cancelled"
		if err := s.eventStore.Update(ctx, ev); err != nil {
			s.logger.Error().Err(err).Str("event_id", ev.ID).Msg("failed to cancel series occurrence")
		}
	}

	return series, nil
}

// GetSeriesWithOccurrences returns a series and its list of occurrences.
func (s *SeriesService) GetSeriesWithOccurrences(ctx context.Context, seriesID, organizerID string) (*EventSeries, []*Event, error) {
	series, err := s.seriesStore.FindByID(ctx, seriesID)
	if err != nil {
		return nil, nil, err
	}
	if series == nil {
		return nil, nil, fmt.Errorf("series not found")
	}
	if series.OrganizerID != organizerID {
		return nil, nil, fmt.Errorf("forbidden: you do not own this series")
	}

	occurrences, err := s.eventStore.FindBySeriesID(ctx, seriesID)
	if err != nil {
		return nil, nil, err
	}
	if occurrences == nil {
		occurrences = []*Event{}
	}

	return series, occurrences, nil
}

// ListByOrganizer retrieves all series belonging to the given organizer.
func (s *SeriesService) ListByOrganizer(ctx context.Context, organizerID string) ([]*EventSeries, error) {
	series, err := s.seriesStore.FindByOrganizerID(ctx, organizerID)
	if err != nil {
		return nil, err
	}
	if series == nil {
		series = []*EventSeries{}
	}
	return series, nil
}

// DeleteSeries removes a series and disassociates its events.
func (s *SeriesService) DeleteSeries(ctx context.Context, seriesID, organizerID string) error {
	series, err := s.seriesStore.FindByID(ctx, seriesID)
	if err != nil {
		return err
	}
	if series == nil {
		return fmt.Errorf("series not found")
	}
	if series.OrganizerID != organizerID {
		return fmt.Errorf("forbidden: you do not own this series")
	}

	// The foreign key has ON DELETE SET NULL, so deleting the series will
	// set series_id to NULL on all linked events.
	return s.seriesStore.Delete(ctx, seriesID)
}

// buildOccurrenceFromSeries creates an Event struct from a series template
// for the given date and index.
func (s *SeriesService) buildOccurrenceFromSeries(series *EventSeries, eventDate time.Time, index int) *Event {
	shareToken, err := generateBase62Token(8)
	if err != nil {
		// Extremely unlikely; fall back to a UUID prefix.
		shareToken = uuid.Must(uuid.NewV7()).String()[:8]
	}

	var endDate *time.Time
	if series.DurationMinutes != nil && *series.DurationMinutes > 0 {
		t := eventDate.Add(time.Duration(*series.DurationMinutes) * time.Minute)
		endDate = &t
	}

	var rsvpDeadline *time.Time
	if series.RSVPDeadlineOffsetHours != nil && *series.RSVPDeadlineOffsetHours > 0 {
		t := eventDate.Add(-time.Duration(*series.RSVPDeadlineOffsetHours) * time.Hour)
		rsvpDeadline = &t
	}

	seriesID := series.ID
	waitlistEnabled := series.MaxCapacity != nil

	return &Event{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		OrganizerID:        series.OrganizerID,
		Title:              series.Title,
		Description:        series.Description,
		EventDate:          eventDate,
		EndDate:            endDate,
		Location:           series.Location,
		Timezone:           series.Timezone,
		RetentionDays:      series.RetentionDays,
		Status:             "published",
		ShareToken:         shareToken,
		ContactRequirement: series.ContactRequirement,
		ShowHeadcount:      series.ShowHeadcount,
		ShowGuestList:      series.ShowGuestList,
		RSVPDeadline:       rsvpDeadline,
		MaxCapacity:        series.MaxCapacity,
		WaitlistEnabled:    waitlistEnabled,
		SeriesID:           &seriesID,
		SeriesIndex:        &index,
		SeriesOverride:     false,
	}
}

// calculateNextOccurrence computes the next occurrence date based on the
// recurrence rule. For monthly recurrence, it clamps the day to the last
// day of the target month to handle months with fewer days (e.g., Jan 31
// becomes Feb 28).
func calculateNextOccurrence(from time.Time, rule string, originalDay int) time.Time {
	switch rule {
	case "weekly":
		return from.AddDate(0, 0, 7)
	case "biweekly":
		return from.AddDate(0, 0, 14)
	case "monthly":
		y, m, _ := from.Date()
		nextMonth := m + 1
		nextYear := y
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}
		// Find the last day of the next month.
		lastDay := time.Date(nextYear, nextMonth+1, 0, 0, 0, 0, 0, from.Location()).Day()
		day := originalDay
		if day > lastDay {
			day = lastDay
		}
		return time.Date(nextYear, nextMonth, day,
			from.Hour(), from.Minute(), from.Second(), 0, from.Location())
	}
	return from
}

// isValidRecurrenceRule checks whether the given value is a supported
// recurrence rule.
func isValidRecurrenceRule(s string) bool {
	switch s {
	case "weekly", "biweekly", "monthly":
		return true
	default:
		return false
	}
}
