package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidNotificationID = errors.New("notification_id must be a valid UUID")
	ErrInvalidRecipientID    = errors.New("recipient_id must be a valid UUID")
	ErrInvalidSourceEventID  = errors.New("source_event_id must be a valid UUID")
	ErrInvalidType           = errors.New("type must be 1 to 50 characters")
	ErrInvalidTitle          = errors.New("title must be 1 to 150 characters")
	ErrInvalidBody           = errors.New("body must be 1 to 1000 characters")
	ErrNotFound              = errors.New("notification not found")
	ErrNotOwner              = errors.New("caller is not the recipient of this notification")
)

type Notification struct {
	NotificationID string
	RecipientID    string
	Type           string
	Title          string
	Body           string
	RelatedID      string
	Read           bool
	SourceEventID  string
	CreatedAt      time.Time
}

// NewNotification validates the inputs for the internal CreateNotification RPC.
func NewNotification(recipientID, nType, title, body, relatedID, sourceEventID string, now time.Time) (Notification, error) {
	if !IsUUID(recipientID) {
		return Notification{}, ErrInvalidRecipientID
	}
	if !IsUUID(sourceEventID) {
		return Notification{}, ErrInvalidSourceEventID
	}

	nType = strings.TrimSpace(nType)
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if utf8.RuneCountInString(nType) == 0 || utf8.RuneCountInString(nType) > 50 {
		return Notification{}, ErrInvalidType
	}
	if utf8.RuneCountInString(title) == 0 || utf8.RuneCountInString(title) > 150 {
		return Notification{}, ErrInvalidTitle
	}
	if utf8.RuneCountInString(body) == 0 || utf8.RuneCountInString(body) > 1000 {
		return Notification{}, ErrInvalidBody
	}

	return Notification{
		RecipientID:   recipientID,
		Type:          nType,
		Title:         title,
		Body:          body,
		RelatedID:     relatedID,
		SourceEventID: sourceEventID,
		Read:          false,
		CreatedAt:     now,
	}, nil
}
