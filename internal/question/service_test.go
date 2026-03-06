package question

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openrsvp/openrsvp/internal/auth"
	"github.com/openrsvp/openrsvp/internal/event"
	"github.com/openrsvp/openrsvp/internal/testutil"
)

// setupQuestion creates a test DB with an organizer and event, returning
// the question service, event ID, and organizer ID.
func setupQuestion(t *testing.T) (*Service, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	store := NewStore(db)
	svc := NewService(store)
	return svc, ev.ID, org.ID
}

// setupQuestionWithAttendee creates a test DB with an organizer, event, and
// attendee, returning the question service, event ID, and attendee ID.
func setupQuestionWithAttendee(t *testing.T) (*Service, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	cfg := testutil.TestConfig()

	authStore := auth.NewStore(db)
	org, err := authStore.CreateOrganizer(context.Background(), "org@example.com")
	require.NoError(t, err)

	eventStore := event.NewStore(db)
	eventSvc := event.NewService(eventStore, cfg.DefaultRetentionDays)
	ev, err := eventSvc.Create(context.Background(), org.ID, event.CreateEventRequest{
		Title: "Test Event", EventDate: "2026-06-15T14:00",
	})
	require.NoError(t, err)

	// Create an attendee.
	email := "attendee@example.com"
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO attendees (id, event_id, name, email, rsvp_status, rsvp_token, contact_method, dietary_notes, plus_ones, created_at, updated_at)
		 VALUES ('att-001', ?, 'Test Attendee', ?, 'attending', 'tok123', 'email', '', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		ev.ID, email,
	)
	require.NoError(t, err)

	store := NewStore(db)
	svc := NewService(store)
	return svc, ev.ID, "att-001"
}

func TestCreateQuestion_Text(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "What is your favorite color?",
		Type:  "text",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, q.ID)
	assert.Equal(t, eventID, q.EventID)
	assert.Equal(t, "What is your favorite color?", q.Label)
	assert.Equal(t, "text", q.Type)
	assert.Empty(t, q.Options)
	assert.False(t, q.Required)
	assert.Equal(t, 0, q.SortOrder)
}

func TestCreateQuestion_Select(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	required := true
	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:    "Meal preference",
		Type:     "select",
		Options:  []string{"Chicken", "Fish", "Vegetarian"},
		Required: &required,
	})
	require.NoError(t, err)
	assert.Equal(t, "select", q.Type)
	assert.Equal(t, []string{"Chicken", "Fish", "Vegetarian"}, q.Options)
	assert.True(t, q.Required)
}

func TestCreateQuestion_Select_TooFewOptions(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Choose one",
		Type:    "select",
		Options: []string{"Only one"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 options")
}

func TestCreateQuestion_MaxReached(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	// Create 10 questions (the max).
	for i := 0; i < maxQuestionsPerEvent; i++ {
		_, err := svc.Create(ctx, eventID, CreateQuestionRequest{
			Label: "Question",
			Type:  "text",
		})
		require.NoError(t, err)
	}

	// 11th should fail.
	_, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "One too many",
		Type:  "text",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

func TestCreateQuestion_InvalidType(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "Bad type",
		Type:  "radio",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid question type")
}

func TestCreateQuestion_EmptyLabel(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "",
		Type:  "text",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "label is required")
}

func TestValidateAnswers_RequiredMissing(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	required := true
	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:    "Must answer this",
		Type:     "text",
		Required: &required,
	})
	require.NoError(t, err)

	// Submit without answering the required question.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "answer required")

	// Submit with an empty answer.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "answer required")
}

func TestValidateAnswers_InvalidOption(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Pick one",
		Type:    "select",
		Options: []string{"A", "B", "C"},
	})
	require.NoError(t, err)

	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "D",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid option")
}

func TestValidateAnswers_CheckboxFormat(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Select all that apply",
		Type:    "checkbox",
		Options: []string{"Red", "Green", "Blue"},
	})
	require.NoError(t, err)

	// Valid checkbox answer (JSON array).
	selectedJSON, _ := json.Marshal([]string{"Red", "Blue"})
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: string(selectedJSON),
	})
	assert.NoError(t, err)

	// Invalid: element not in options.
	badJSON, _ := json.Marshal([]string{"Red", "Yellow"})
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: string(badJSON),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid option")

	// Invalid: not a JSON array.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "not json",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON array")
}

func TestValidateAnswers_TextTooLong(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "Tell us about yourself",
		Type:  "text",
	})
	require.NoError(t, err)

	longText := strings.Repeat("x", maxTextAnswerLength+1)
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: longText,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestSoftDeleteQuestion_AnswersPreserved(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "Will be deleted",
		Type:  "text",
	})
	require.NoError(t, err)

	// Submit an answer.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "my answer",
	})
	require.NoError(t, err)

	// Soft-delete the question.
	err = svc.Delete(ctx, q.ID)
	require.NoError(t, err)

	// Question should not appear in list.
	questions, err := svc.ListByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, questions, 0)

	// But answers should still exist.
	answers, err := svc.GetAnswersForAttendee(ctx, attendeeID)
	require.NoError(t, err)
	assert.Len(t, answers, 1)
	assert.Equal(t, "my answer", answers[0].Answer)
}

func TestReorderQuestions(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	q1, err := svc.Create(ctx, eventID, CreateQuestionRequest{Label: "First", Type: "text"})
	require.NoError(t, err)
	q2, err := svc.Create(ctx, eventID, CreateQuestionRequest{Label: "Second", Type: "text"})
	require.NoError(t, err)
	q3, err := svc.Create(ctx, eventID, CreateQuestionRequest{Label: "Third", Type: "text"})
	require.NoError(t, err)

	// Reorder: q3, q1, q2.
	err = svc.Reorder(ctx, eventID, []string{q3.ID, q1.ID, q2.ID})
	require.NoError(t, err)

	// Verify the new order.
	questions, err := svc.ListByEvent(ctx, eventID)
	require.NoError(t, err)
	require.Len(t, questions, 3)
	assert.Equal(t, q3.ID, questions[0].ID)
	assert.Equal(t, q1.ID, questions[1].ID)
	assert.Equal(t, q2.ID, questions[2].ID)
	assert.Equal(t, 0, questions[0].SortOrder)
	assert.Equal(t, 1, questions[1].SortOrder)
	assert.Equal(t, 2, questions[2].SortOrder)
}

func TestUpsertAnswer(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "Your name?",
		Type:  "text",
	})
	require.NoError(t, err)

	// First answer.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "Alice",
	})
	require.NoError(t, err)

	answers, err := svc.GetAnswersForAttendee(ctx, attendeeID)
	require.NoError(t, err)
	require.Len(t, answers, 1)
	assert.Equal(t, "Alice", answers[0].Answer)

	// Update the answer.
	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "Bob",
	})
	require.NoError(t, err)

	answers, err = svc.GetAnswersForAttendee(ctx, attendeeID)
	require.NoError(t, err)
	require.Len(t, answers, 1)
	assert.Equal(t, "Bob", answers[0].Answer)
}

func TestValidateAnswers_ValidSelectOption(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Meal",
		Type:    "select",
		Options: []string{"Chicken", "Fish"},
	})
	require.NoError(t, err)

	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "Chicken",
	})
	assert.NoError(t, err)

	answers, err := svc.GetAnswersForAttendee(ctx, attendeeID)
	require.NoError(t, err)
	require.Len(t, answers, 1)
	assert.Equal(t, "Chicken", answers[0].Answer)
}

func TestUpdateQuestion(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Original",
		Type:    "select",
		Options: []string{"A", "B"},
	})
	require.NoError(t, err)

	newLabel := "Updated"
	updated, err := svc.Update(ctx, q.ID, UpdateQuestionRequest{
		Label:   &newLabel,
		Options: []string{"X", "Y", "Z"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Label)
	assert.Equal(t, []string{"X", "Y", "Z"}, updated.Options)
}

func TestCreateQuestion_Checkbox(t *testing.T) {
	svc, eventID, _ := setupQuestion(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label:   "Dietary restrictions",
		Type:    "checkbox",
		Options: []string{"Gluten-free", "Dairy-free", "Nut-free"},
	})
	require.NoError(t, err)
	assert.Equal(t, "checkbox", q.Type)
	assert.Len(t, q.Options, 3)
}

func TestGetAnswersByEvent(t *testing.T) {
	svc, eventID, attendeeID := setupQuestionWithAttendee(t)
	ctx := context.Background()

	q, err := svc.Create(ctx, eventID, CreateQuestionRequest{
		Label: "Name",
		Type:  "text",
	})
	require.NoError(t, err)

	err = svc.ValidateAndSaveAnswers(ctx, attendeeID, eventID, map[string]string{
		q.ID: "My Answer",
	})
	require.NoError(t, err)

	answerMap, err := svc.GetAnswersByEvent(ctx, eventID)
	require.NoError(t, err)
	assert.Len(t, answerMap, 1)
	assert.Len(t, answerMap[attendeeID], 1)
	assert.Equal(t, "My Answer", answerMap[attendeeID][0].Answer)
}
