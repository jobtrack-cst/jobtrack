package domain

import (
	"errors"
	"time"
)

// ErrConflictingEvent indicates an event inconsistent with trusted state.
var ErrConflictingEvent = errors.New("application event conflicts with stored state")

// ApplicationEvent is validated application-owned input.
type ApplicationEvent struct {
	EventID, ApplicationID, ApplicantID, EmployerID string
	Submitted                                       bool
	OccurredAt                                      time.Time
}
