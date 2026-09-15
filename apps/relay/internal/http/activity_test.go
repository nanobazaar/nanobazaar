package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nanobazaar/relay/internal/auth"
	"github.com/nanobazaar/relay/internal/store"
)

func TestVerifiedContactUpdatesCallerOnly(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	st := store.New(db)
	pub, priv := generateSigningKey(t)
	caller := seedBotWithKey(t, st, pub)
	otherPub, _ := generateSigningKey(t)
	seller := seedBotWithKey(t, st, otherPub)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-24 * time.Hour)
	if _, err := db.Exec("UPDATE bots SET last_seen_at = ?", old); err != nil {
		t.Fatal(err)
	}
	verifier := auth.NewVerifier(st)
	verifier.Clock = func() time.Time { return now }
	router := NewRouter(RouterConfig{Store: st, Verifier: verifier})
	req := signedRequest(t, priv, caller, http.MethodGet, "/v0/bots/"+seller, "", nil, now.Add(-time.Minute), "contact")
	rec := httptestRequest(t, router, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup: %d %s", rec.Code, rec.Body.String())
	}
	assertBotContact(t, st, caller, now)
	assertBotContact(t, st, seller, old)
	// Replaying the accepted nonce must not count as another contact.
	now = now.Add(2 * time.Minute)
	replay := signedRequest(t, priv, caller, http.MethodGet, "/v0/bots/"+seller, "", nil, now, "contact")
	if got := httptestRequest(t, router, replay); got.Code != http.StatusUnauthorized {
		t.Fatalf("replay: %d", got.Code)
	}
	assertBotContact(t, st, caller, now.Add(-2*time.Minute))
}

func TestRejectedAuthDoesNotUpdateActivity(t *testing.T) {
	for _, scenario := range []string{"invalid signature", "stale request", "registration identity spoof", "registration key conflict"} {
		t.Run(scenario, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			st := store.New(db)
			pub, priv := generateSigningKey(t)
			caller := seedBotWithKey(t, st, pub)
			attackerPub, attackerKey := generateSigningKey(t)
			now := time.Now().UTC().Truncate(time.Second)
			old := now.Add(-24 * time.Hour)
			if _, err := db.Exec("UPDATE bots SET last_seen_at = ?", old); err != nil {
				t.Fatal(err)
			}
			verifier := auth.NewVerifier(st)
			verifier.Clock = func() time.Time { return now }
			var req *http.Request
			want := http.StatusUnauthorized
			switch scenario {
			case "invalid signature":
				req = signedRequest(t, attackerKey, caller, http.MethodGet, "/v0/poll", "", nil, now, "bad")
			case "stale request":
				req = signedRequest(t, priv, caller, http.MethodGet, "/v0/poll", "", nil, now.Add(-time.Hour), "stale")
			default:
				key, signingPub := ed25519.PrivateKey(attackerKey), attackerPub
				want = http.StatusBadRequest
				if scenario == "registration key conflict" {
					key, signingPub = priv, pub
					want = http.StatusConflict
				}
				enc := randomKeyBytes(t)
				body := mustJSONBytes(t, botRegistrationRequest{SigningPubkeyEd25519: base64.RawURLEncoding.EncodeToString(signingPub), EncryptionPubkeyX25519: base64.RawURLEncoding.EncodeToString(enc), SigningKid: kidFromKey(signingPub), EncryptionKid: kidFromKey(enc)})
				req = signedRequest(t, key, caller, http.MethodPost, "/v0/bots", "", body, now, "registration")
				req.Header.Set(headerIdempotency, "registration")
			}
			if rec := httptestRequest(t, NewRouter(RouterConfig{Store: st, Verifier: verifier}), req); rec.Code != want {
				t.Fatalf("want %d: %d %s", want, rec.Code, rec.Body.String())
			}
			assertBotContact(t, st, caller, old)
		})
	}
}

func TestOffersExposeSellerContactWithoutRefreshingIt(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprintf("unknown=%v", unknown), func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			st := store.New(db)
			pub, priv := generateSigningKey(t)
			caller := seedBotWithKey(t, st, pub)
			sellerPub, _ := generateSigningKey(t)
			seller := seedBotWithKey(t, st, sellerPub)
			now := time.Now().UTC().Truncate(time.Second)
			old := now.Add(-24 * time.Hour)
			var stored any = old
			if unknown {
				stored = nil
			}
			if _, err := db.Exec("UPDATE bots SET last_seen_at = ? WHERE bot_id = ?", stored, seller); err != nil {
				t.Fatal(err)
			}
			offerID := createOfferForTest(t, st, seller, "offer_activity", now.Add(-time.Hour))
			verifier := auth.NewVerifier(st)
			verifier.Clock = func() time.Time { return now }
			router := NewRouter(RouterConfig{Store: st, Verifier: verifier})
			for i, route := range []struct {
				path, query  string
				public, list bool
			}{
				{"/v0/offers/" + offerID, "", false, false}, {"/v0/offers", "", false, true}, {"/v0/offers", "q=Test", false, true},
				{"/market/offers/" + offerID, "", true, false}, {"/market/offers", "", true, true}, {"/market/offers", "q=Test", true, true},
			} {
				req := signedRequest(t, priv, caller, http.MethodGet, route.path, route.query, nil, now, fmt.Sprintf("list-%d", i))
				if route.public {
					req = httptest.NewRequest(http.MethodGet, route.path+"?"+route.query, nil)
				}
				rec := httptestRequest(t, router, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("%s: %d %s", route.path, rec.Code, rec.Body.String())
				}
				var data map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
					t.Fatal(err)
				}
				if route.list {
					var offers []map[string]json.RawMessage
					if err := json.Unmarshal(data["offers"], &offers); err != nil {
						t.Fatal(err)
					}
					if len(offers) != 1 {
						t.Fatalf("expected offer: %s", rec.Body.String())
					}
					data = offers[0]
				}
				value, exists := data["seller_last_seen_at"]
				if !exists {
					t.Fatalf("%s missing seller_last_seen_at: %s", route.path, rec.Body.String())
				}
				if unknown {
					if string(value) != "null" {
						t.Fatalf("expected null: %s", value)
					}
				} else {
					var got time.Time
					if err := json.Unmarshal(value, &got); err != nil {
						t.Fatal(err)
					}
					if !got.Equal(old) {
						t.Fatalf("%s refreshed seller: %s", route.path, got)
					}
				}
			}
			bot, err := st.GetBot(context.Background(), seller)
			if err != nil {
				t.Fatal(err)
			}
			if unknown {
				if bot.LastSeenAt.Valid {
					t.Fatal("public reads populated unknown contact")
				}
			} else {
				assertBotContact(t, st, seller, old)
			}
		})
	}
}

func assertBotContact(t *testing.T, st *store.Store, botID string, want time.Time) {
	t.Helper()
	bot, err := st.GetBot(context.Background(), botID)
	if err != nil {
		t.Fatal(err)
	}
	if !bot.LastSeenAt.Valid || !bot.LastSeenAt.Time.Equal(want) {
		t.Fatalf("bot %s contact = %v; want %s", botID, bot.LastSeenAt, want)
	}
}
