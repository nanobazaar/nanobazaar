package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/store"
	"github.com/nanobazaar/relay/internal/store/sqlc"
)

func TestPollAndAckFlow(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	recipient := "bot_recipient"
	seedJobBot(t, st, recipient, now)

	seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
	seedEvent(t, st, recipient, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))

	router := NewRouter(RouterConfig{Store: st})

	pollReq := httptest.NewRequest(http.MethodGet, "/v0/poll?limit=10", nil)
	pollReq.Header.Set(headerBotID, recipient)
	pollRec := httptestRequest(t, router, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pollRec.Code, pollRec.Body.String())
	}
	var pollResp pollResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &pollResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(pollResp.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(pollResp.Events))
	}
	if pollResp.LastAckedEventID != 0 {
		t.Fatalf("expected last_acked_event_id 0, got %d", pollResp.LastAckedEventID)
	}

	ackReq := newJSONRequest(t, http.MethodPost, "/v0/poll/ack", mustJSONBytes(t, pollAckRequest{UpToEventID: pollResp.Events[0].EventID}))
	ackReq.Header.Set(headerBotID, recipient)
	ackRec := httptestRequest(t, router, ackReq)
	if ackRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", ackRec.Code, ackRec.Body.String())
	}
	var ackResp pollAckResponse
	if err := json.Unmarshal(ackRec.Body.Bytes(), &ackResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ackResp.LastAckedEventID != pollResp.Events[0].EventID {
		t.Fatalf("expected last_acked_event_id updated")
	}

	pollReq2 := httptest.NewRequest(http.MethodGet, "/v0/poll", nil)
	pollReq2.Header.Set(headerBotID, recipient)
	pollRec2 := httptestRequest(t, router, pollReq2)
	if pollRec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pollRec2.Code, pollRec2.Body.String())
	}
	var pollResp2 pollResponse
	if err := json.Unmarshal(pollRec2.Body.Bytes(), &pollResp2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pollResp2.LastAckedEventID != ackResp.LastAckedEventID {
		t.Fatalf("expected last_acked_event_id propagated")
	}
}

func TestPollTypesFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	recipient := "bot_recipient"
	seedJobBot(t, st, recipient, now)

	seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
	seedEvent(t, st, recipient, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))

	router := NewRouter(RouterConfig{Store: st})
	pollReq := httptest.NewRequest(http.MethodGet, "/v0/poll?types=job.paid", nil)
	pollReq.Header.Set(headerBotID, recipient)
	pollRec := httptestRequest(t, router, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pollRec.Code, pollRec.Body.String())
	}
	var pollResp pollResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &pollResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(pollResp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pollResp.Events))
	}
	if pollResp.Events[0].EventType != "job.paid" {
		t.Fatalf("unexpected event type %s", pollResp.Events[0].EventType)
	}
}

func TestPollGoneWhenCursorTooOld(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	recipient := "bot_recipient"
	seedJobBot(t, st, recipient, now)

	seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
	seedEvent(t, st, recipient, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))
	seedEvent(t, st, recipient, "job.cancelled", map[string]any{"job_id": "job_1"}, now.Add(2*time.Minute))

	if _, err := db.Exec("DELETE FROM events WHERE event_id < 3"); err != nil {
		t.Fatalf("delete events: %v", err)
	}

	router := NewRouter(RouterConfig{Store: st})
	pollReq := httptest.NewRequest(http.MethodGet, "/v0/poll?since_event_id=0", nil)
	pollReq.Header.Set(headerBotID, recipient)
	pollRec := httptestRequest(t, router, pollReq)
	if pollRec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d: %s", pollRec.Code, pollRec.Body.String())
	}
	var gone pollGoneResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &gone); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gone.MinEventIDRetained != 3 {
		t.Fatalf("expected min_event_id_retained 3, got %d", gone.MinEventIDRetained)
	}
	if !gone.SuggestedResync {
		t.Fatalf("expected suggested_resync true")
	}
}

func TestPollSinceEventIDReturnsStoredAck(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	recipient := "bot_recipient"
	seedJobBot(t, st, recipient, now)

	seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
	seedEvent(t, st, recipient, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))
	seedEvent(t, st, recipient, "job.cancelled", map[string]any{"job_id": "job_1"}, now.Add(2*time.Minute))

	if err := st.UpsertPollAck(context.Background(), sqlc.UpsertPollAckParams{
		RecipientBotID:   recipient,
		LastAckedEventID: 3,
		UpdatedAt:        now,
	}); err != nil {
		t.Fatalf("upsert poll ack: %v", err)
	}

	router := NewRouter(RouterConfig{Store: st})
	pollReq := httptest.NewRequest(http.MethodGet, "/v0/poll?since_event_id=1", nil)
	pollReq.Header.Set(headerBotID, recipient)
	pollRec := httptestRequest(t, router, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pollRec.Code, pollRec.Body.String())
	}

	var pollResp pollResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &pollResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pollResp.LastAckedEventID != 3 {
		t.Fatalf("expected last_acked_event_id 3, got %d", pollResp.LastAckedEventID)
	}
	if len(pollResp.Events) == 0 || pollResp.Events[0].EventID <= 1 {
		t.Fatalf("expected events after since_event_id")
	}
}

func TestPollSinceEventIDNoStoredAckReturnsZero(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	st := store.New(db)
	now := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	recipient := "bot_recipient"
	seedJobBot(t, st, recipient, now)

	seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
	seedEvent(t, st, recipient, "job.paid", map[string]any{"job_id": "job_1"}, now.Add(time.Minute))

	router := NewRouter(RouterConfig{Store: st})
	pollReq := httptest.NewRequest(http.MethodGet, "/v0/poll?since_event_id=1", nil)
	pollReq.Header.Set(headerBotID, recipient)
	pollRec := httptestRequest(t, router, pollReq)
	if pollRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", pollRec.Code, pollRec.Body.String())
	}

	var pollResp pollResponse
	if err := json.Unmarshal(pollRec.Body.Bytes(), &pollResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pollResp.LastAckedEventID != 0 {
		t.Fatalf("expected last_acked_event_id 0, got %d", pollResp.LastAckedEventID)
	}
}

// CONTRACT.md, Polling and ack: global event ID gaps do not prove that this
// recipient lost events. A deleted recipient event does, including an empty log.
func TestPollRetentionIsPerRecipient(t *testing.T) {
	tests := []struct {
		name       string
		recipients []string
		deleted    []int64
		cursor     int64
		wantStatus int
		wantMin    int64
		wantEvents []int64
	}{
		{name: "empty fresh bot", wantStatus: 200},
		{name: "fresh bot first event has high ID", recipients: []string{"other", "other", "other", "recipient"}, wantStatus: 200, wantMin: 4, wantEvents: []int64{4}},
		{name: "recipient IDs have gaps", recipients: []string{"other", "recipient", "other", "recipient"}, cursor: 2, wantStatus: 200, wantMin: 2, wantEvents: []int64{4}},
		{name: "other recipient deletion", recipients: []string{"other", "other", "recipient"}, deleted: []int64{1, 2}, wantStatus: 200, wantMin: 3, wantEvents: []int64{3}},
		{name: "deleted event makes old cursor gone", recipients: []string{"recipient", "other", "recipient"}, deleted: []int64{1}, wantStatus: 410, wantMin: 3},
		{name: "cursor equal to watermark is safe", recipients: []string{"recipient", "other", "recipient"}, deleted: []int64{1}, cursor: 1, wantStatus: 200, wantMin: 3, wantEvents: []int64{3}},
		{name: "all events deleted", recipients: []string{"other", "recipient"}, deleted: []int64{2}, wantStatus: 410, wantMin: 3},
		{name: "empty log after resync", recipients: []string{"other", "recipient"}, deleted: []int64{2}, cursor: 2, wantStatus: 200, wantMin: 3},
		{name: "non-prefix deletion", recipients: []string{"recipient", "recipient", "recipient"}, deleted: []int64{2}, wantStatus: 410, wantMin: 3},
		{name: "older deletion cannot lower watermark", recipients: []string{"recipient", "recipient", "recipient"}, deleted: []int64{3, 1}, cursor: 2, wantStatus: 410, wantMin: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			st := store.New(db)
			now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
			seedJobBot(t, st, "recipient", now)
			seedJobBot(t, st, "other", now)
			for _, recipient := range tt.recipients {
				seedEvent(t, st, recipient, "job.requested", map[string]any{"job_id": "job_1"}, now)
			}
			for _, id := range tt.deleted {
				if _, err := db.Exec("DELETE FROM events WHERE event_id = ?", id); err != nil {
					t.Fatal(err)
				}
			}
			router := NewRouter(RouterConfig{Store: st})
			req := httptest.NewRequest(http.MethodGet, "/v0/poll?since_event_id="+strconv.FormatInt(tt.cursor, 10), nil)
			req.Header.Set(headerBotID, "recipient")
			rec := httptestRequest(t, router, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var response pollResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.MinEventIDRetained != tt.wantMin {
				t.Fatalf("min_event_id_retained = %d, want %d", response.MinEventIDRetained, tt.wantMin)
			}
			if tt.wantStatus == http.StatusGone {
				// The contract's min-1 resync ACK must make a subsequent poll usable.
				ackReq := newJSONRequest(t, http.MethodPost, "/v0/poll/ack", mustJSONBytes(t, pollAckRequest{UpToEventID: response.MinEventIDRetained - 1}))
				ackReq.Header.Set(headerBotID, "recipient")
				if ack := httptestRequest(t, router, ackReq); ack.Code != http.StatusOK {
					t.Fatalf("resync ack: %d %s", ack.Code, ack.Body.String())
				}
				req = httptest.NewRequest(http.MethodGet, "/v0/poll", nil)
				req.Header.Set(headerBotID, "recipient")
				if resumed := httptestRequest(t, router, req); resumed.Code != http.StatusOK {
					t.Fatalf("poll after resync: %d %s", resumed.Code, resumed.Body.String())
				}
				return
			}
			if len(response.Events) != len(tt.wantEvents) {
				t.Fatalf("events = %v, want IDs %v", response.Events, tt.wantEvents)
			}
			for i, event := range response.Events {
				if event.EventID != tt.wantEvents[i] {
					t.Fatalf("event[%d] = %d, want %d", i, event.EventID, tt.wantEvents[i])
				}
			}
		})
	}
}

func seedEvent(t *testing.T, st *store.Store, recipient, eventType string, data map[string]any, createdAt time.Time) {
	t.Helper()
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = st.CreateEvent(context.Background(), sqlc.CreateEventParams{
		RecipientBotID: recipient,
		EventType:      eventType,
		DataJson:       string(payload),
		CreatedAt:      createdAt,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
}
