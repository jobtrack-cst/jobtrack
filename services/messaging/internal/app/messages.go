package app

import (
	"context"
	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/ports"
)

type Messages struct {
	repo         ports.MessageRepository
	participants ports.ParticipantRepository
	clock        ports.Clock
}

func NewMessages(r ports.MessageRepository, p ports.ParticipantRepository, c ports.Clock) *Messages {
	return &Messages{r, p, c}
}
func (s *Messages) authorize(ctx context.Context, c domain.Caller, raw string) (domain.ApplicationParticipants, string, error) {
	c, err := domain.NewCaller(c.UserID, c.Role)
	if err != nil {
		return domain.ApplicationParticipants{}, "", err
	}
	if c.Role == "admin" {
		return domain.ApplicationParticipants{}, "", domain.ErrPermissionDenied
	}
	id, err := domain.NormalizeID(raw, domain.ErrInvalidApplicationID)
	if err != nil {
		return domain.ApplicationParticipants{}, "", err
	}
	p, err := s.participants.GetByApplicationID(ctx, id)
	if err != nil {
		return p, "", err
	}
	recipient, err := p.Recipient(c)
	return p, recipient, err
}
func (s *Messages) Send(ctx context.Context, c domain.Caller, id, raw string) (domain.Message, error) {
	p, recipient, err := s.authorize(ctx, c, id)
	if err != nil {
		return domain.Message{}, err
	}
	content, err := domain.ValidateContent(raw)
	if err != nil {
		return domain.Message{}, err
	}
	sender, _ := domain.NormalizeID(c.UserID, domain.ErrUnauthenticated)
	now := s.clock.Now().UTC()
	return s.repo.Create(ctx, domain.Message{ID: uuid.NewString(), ApplicationID: p.ApplicationID, SenderID: sender, RecipientID: recipient, Content: content, CreatedAt: now, UpdatedAt: now})
}
func (s *Messages) List(ctx context.Context, c domain.Caller, id string, size int32, token string) ([]domain.Message, int64, domain.Page, error) {
	p, _, err := s.authorize(ctx, c, id)
	if err != nil {
		return nil, 0, domain.Page{}, err
	}
	page, err := domain.ParsePage(size, token)
	if err != nil {
		return nil, 0, page, err
	}
	messages, total, err := s.repo.List(ctx, p.ApplicationID, page)
	return messages, total, page, err
}
