package ports

//go:generate go tool mockgen -destination=mocks/messages_mock.go -package=mocks . MessageRepository,Clock
import (
	"context"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"time"
)

// MessageRepository stores one logical thread per application.
type MessageRepository interface {
	Create(context.Context, domain.Message) (domain.Message, error)
	// List returns oldest first, breaking timestamp ties by message ID. Count and
	// rows must come from the same snapshot; offsets beyond the end return no rows.
	List(context.Context, string, domain.Page) ([]domain.Message, int64, error)
}
type Clock interface{ Now() time.Time }
