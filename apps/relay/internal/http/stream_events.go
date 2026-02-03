package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/nanobazaar/relay/internal/store/sqlc"
)

type streamEventQuerier interface {
	CreateStreamEvent(ctx context.Context, arg sqlc.CreateStreamEventParams) (int64, error)
	GetBot(ctx context.Context, botID string) (sqlc.Bot, error)
}

func emitStreamEvents(ctx context.Context, q streamEventQuerier, recipient, eventType, payloadJSON string, data map[string]any, createdAt time.Time) error {
	if recipient == "" {
		return errors.New("missing recipient")
	}
	if q == nil {
		return errors.New("stream event store unavailable")
	}
	bot, err := q.GetBot(ctx, recipient)
	if err != nil {
		return err
	}
	if bot.SigningPubkeyEd25519 == "" {
		return errors.New("missing signing pubkey")
	}
	if _, err := q.CreateStreamEvent(ctx, sqlc.CreateStreamEventParams{
		StreamKey:   "seller:ed25519:" + bot.SigningPubkeyEd25519,
		EventType:   eventType,
		CreatedAt:   createdAt,
		PayloadJson: payloadJSON,
	}); err != nil {
		return err
	}
	if data != nil {
		if jobID, ok := data["job_id"].(string); ok && jobID != "" {
			if _, err := q.CreateStreamEvent(ctx, sqlc.CreateStreamEventParams{
				StreamKey:   "job:" + jobID,
				EventType:   eventType,
				CreatedAt:   createdAt,
				PayloadJson: payloadJSON,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
