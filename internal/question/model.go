package question

import "time"

// Question represents a custom question attached to an event.
type Question struct {
	ID        string    `json:"id"`
	EventID   string    `json:"eventId"`
	Label     string    `json:"label"`
	Type      string    `json:"type"`      // text, select, checkbox
	Options   []string  `json:"options"`   // For select/checkbox
	Required  bool      `json:"required"`
	SortOrder int       `json:"sortOrder"`
	Deleted   bool      `json:"-"` // Hidden from API
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CreateQuestionRequest is the request body for creating a new question.
type CreateQuestionRequest struct {
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Options   []string `json:"options,omitempty"`
	Required  *bool    `json:"required,omitempty"`
	SortOrder *int     `json:"sortOrder,omitempty"`
}

// UpdateQuestionRequest is the request body for updating an existing question.
type UpdateQuestionRequest struct {
	Label     *string  `json:"label,omitempty"`
	Type      *string  `json:"type,omitempty"`
	Options   []string `json:"options,omitempty"`
	Required  *bool    `json:"required,omitempty"`
	SortOrder *int     `json:"sortOrder,omitempty"`
}

// Answer represents an attendee's answer to a question.
type Answer struct {
	ID         string    `json:"id"`
	AttendeeID string    `json:"attendeeId"`
	QuestionID string    `json:"questionId"`
	Answer     string    `json:"answer"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// SubmitAnswersRequest is the request body for submitting answers.
type SubmitAnswersRequest struct {
	Answers map[string]string `json:"answers"` // questionID -> answer
}
