package events

import "context"

type Event struct {
	EventID int64
	Type    string
	Data    any
}

type Publisher interface {
	Enqueue(ctx context.Context, recipientBotID string, event Event) error
}
