package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// mockSettings – implements stripeSettingReader for tests (no DB needed)
// ---------------------------------------------------------------------------

type mockSettings struct {
	secretKey     string
	webhookSecret string
	priceID       string
	enabled       bool
}

func (m *mockSettings) GetStripeEnable() (bool, error)        { return m.enabled, nil }
func (m *mockSettings) GetStripeSecretKey() (string, error)   { return m.secretKey, nil }
func (m *mockSettings) GetStripeWebhookSecret() (string, error) { return m.webhookSecret, nil }
func (m *mockSettings) GetStripePriceID() (string, error)     { return m.priceID, nil }

// newTestStripeService creates a StripeService with fake settings (no DB / network).
func newTestStripeService(secretKey string) *StripeService {
	return &StripeService{
		settingService: &mockSettings{
			secretKey: secretKey,
			enabled:   secretKey != "",
			priceID:   "price_mock",
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildSigHeader constructs a valid Stripe-Signature header value for the
// given payload and secret – mirrors exactly what Stripe sends.
func buildSigHeader(t *testing.T, payload []byte, secret string) string {
	t.Helper()
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(payload)))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

// withTestServer temporarily replaces stripeAPIBase with the test server URL,
// runs fn, then restores the original value.
func withTestServer(ts *httptest.Server, fn func(svc *StripeService)) {
	original := stripeAPIBase
	stripeAPIBase = ts.URL
	defer func() { stripeAPIBase = original }()
	fn(newTestStripeService("sk_test_fake"))
}

// ---------------------------------------------------------------------------
// VerifyWebhookSignature
// ---------------------------------------------------------------------------

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	svc := &StripeService{}
	secret := "whsec_testsecret123"
	payload := []byte(`{"id":"evt_1","type":"invoice.paid"}`)
	sigHeader := buildSigHeader(t, payload, secret)

	if err := svc.VerifyWebhookSignature(payload, sigHeader, secret); err != nil {
		t.Fatalf("expected valid signature but got error: %v", err)
	}
}

func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	svc := &StripeService{}
	secret := "whsec_testsecret123"
	original := []byte(`{"id":"evt_1","type":"invoice.paid"}`)
	sigHeader := buildSigHeader(t, original, secret)

	tampered := []byte(`{"id":"evt_1","type":"invoice.payment_failed"}`)
	if err := svc.VerifyWebhookSignature(tampered, sigHeader, secret); err == nil {
		t.Fatal("expected signature mismatch but got nil")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	svc := &StripeService{}
	payload := []byte(`{"id":"evt_1","type":"invoice.paid"}`)
	sigHeader := buildSigHeader(t, payload, "correct_secret")

	if err := svc.VerifyWebhookSignature(payload, sigHeader, "wrong_secret"); err == nil {
		t.Fatal("expected signature mismatch but got nil")
	}
}

func TestVerifyWebhookSignature_EmptyHeader(t *testing.T) {
	svc := &StripeService{}
	if err := svc.VerifyWebhookSignature([]byte("{}"), "", "secret"); err == nil {
		t.Fatal("expected error for empty header but got nil")
	}
}

func TestVerifyWebhookSignature_MalformedHeader(t *testing.T) {
	svc := &StripeService{}
	if err := svc.VerifyWebhookSignature([]byte("{}"), "not-valid", "secret"); err == nil {
		t.Fatal("expected error for malformed header but got nil")
	}
}

func TestVerifyWebhookSignature_MultipleV1OneValid(t *testing.T) {
	svc := &StripeService{}
	secret := "whsec_multi"
	payload := []byte(`{"id":"evt_2"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(payload)))
	validSig := hex.EncodeToString(mac.Sum(nil))

	// Stripe sends multiple v1 values when rolling signing secrets
	sigHeader := fmt.Sprintf("t=%s,v1=deadbeef,v1=%s", ts, validSig)
	if err := svc.VerifyWebhookSignature(payload, sigHeader, secret); err != nil {
		t.Fatalf("expected success when one of multiple v1 sigs matches: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HandleWebhookEvent
// ---------------------------------------------------------------------------

func TestHandleWebhookEvent_UnknownType_NoError(t *testing.T) {
	svc := &StripeService{}
	event := map[string]any{
		"id":   "evt_unknown",
		"type": "payment_intent.created",
		"data": map[string]any{"object": map[string]any{}},
	}
	payload, _ := json.Marshal(event)
	// Unknown events must be silently ignored (no error).
	if err := svc.HandleWebhookEvent(payload); err != nil {
		t.Fatalf("unexpected error for unknown event type: %v", err)
	}
}

func TestHandleWebhookEvent_InvalidJSON(t *testing.T) {
	svc := &StripeService{}
	if err := svc.HandleWebhookEvent([]byte("not-json")); err == nil {
		t.Fatal("expected parse error for malformed JSON but got nil")
	}
}

// ---------------------------------------------------------------------------
// doRequest – no secret key configured
// ---------------------------------------------------------------------------

func TestDoRequest_NoSecretKey(t *testing.T) {
	svc := newTestStripeService("") // empty key
	_, err := svc.doRequest(http.MethodPost, "/customers", nil)
	if err == nil {
		t.Fatal("expected error when no secret key configured but got nil")
	}
}

// ---------------------------------------------------------------------------
// CreateCustomer
// ---------------------------------------------------------------------------

func TestCreateCustomer_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/customers" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if got := r.FormValue("email"); got != "alice@example.com" {
			t.Errorf("expected email=alice@example.com, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"id":"cus_test123","email":"alice@example.com"}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		customerID, err := svc.CreateCustomer("alice@example.com")
		if err != nil {
			t.Fatalf("CreateCustomer returned error: %v", err)
		}
		if customerID != "cus_test123" {
			t.Errorf("expected cus_test123, got %s", customerID)
		}
	})
}

func TestCreateCustomer_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"error":{"message":"No such API key"}}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		if _, err := svc.CreateCustomer("bob@example.com"); err == nil {
			t.Fatal("expected error for 401 response but got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// CreateSubscription
// ---------------------------------------------------------------------------

func TestCreateSubscription_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if got := r.FormValue("customer"); got != "cus_abc" {
			t.Errorf("expected customer=cus_abc, got %q", got)
		}
		if got := r.FormValue("items[0][price]"); got != "price_xyz" {
			t.Errorf("expected price_xyz, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"id":"sub_test456","status":"incomplete"}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		subID, status, err := svc.CreateSubscription("cus_abc", "price_xyz")
		if err != nil {
			t.Fatalf("CreateSubscription returned error: %v", err)
		}
		if subID != "sub_test456" {
			t.Errorf("expected sub_test456, got %s", subID)
		}
		if status != "incomplete" {
			t.Errorf("expected status=incomplete, got %s", status)
		}
	})
}

func TestCreateSubscription_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, `{"error":{"message":"No such price"}}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		if _, _, err := svc.CreateSubscription("cus_abc", "price_bad"); err == nil {
			t.Fatal("expected error for 400 response but got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// CancelSubscription
// ---------------------------------------------------------------------------

func TestCancelSubscription_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subscriptions/sub_cancel/cancel" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"id":"sub_cancel","status":"canceled"}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		if err := svc.CancelSubscription("sub_cancel"); err != nil {
			t.Fatalf("CancelSubscription returned error: %v", err)
		}
	})
}

func TestCancelSubscription_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"error":{"message":"No such subscription"}}`)
	}))
	defer ts.Close()

	withTestServer(ts, func(svc *StripeService) {
		if err := svc.CancelSubscription("sub_nonexistent"); err == nil {
			t.Fatal("expected error for 404 response but got nil")
		}
	})
}
