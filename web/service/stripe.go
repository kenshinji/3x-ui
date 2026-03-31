// Package service - stripe.go provides Stripe payment integration for the 3x-ui panel.
// It uses Stripe's REST API directly (no SDK dependency) to create customers and
// subscriptions when VPN clients are added, and handles webhook events to keep
// client enable/disable state in sync with subscription payment status.
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const stripeAPIBase = "https://api.stripe.com/v1"

// StripeService handles Stripe customer and subscription management.
type StripeService struct {
	settingService SettingService
}

// stripeCustomer is the minimal Stripe Customer object we care about.
type stripeCustomer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// stripeSubscription is the minimal Stripe Subscription object we care about.
type stripeSubscription struct {
	ID     string `json:"id"`
	Status string `json:"status"` // active, past_due, canceled, incomplete, etc.
}

// stripeWebhookEvent is the outer envelope for a Stripe webhook event.
type stripeWebhookEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

// doRequest sends an authenticated form-encoded request to the Stripe API.
func (s *StripeService) doRequest(method, endpoint string, params url.Values) ([]byte, error) {
	secretKey, err := s.settingService.GetStripeSecretKey()
	if err != nil || secretKey == "" {
		return nil, fmt.Errorf("stripe secret key not configured")
	}

	var body io.Reader
	var reqURL = stripeAPIBase + endpoint

	if method == http.MethodPost && params != nil {
		body = strings.NewReader(params.Encode())
	} else if method == http.MethodGet && params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(secretKey, "")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stripe API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// CreateCustomer creates a Stripe Customer for the given client email.
// Returns the Stripe customer ID on success.
func (s *StripeService) CreateCustomer(email string) (string, error) {
	params := url.Values{}
	params.Set("email", email)

	data, err := s.doRequest(http.MethodPost, "/customers", params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}

	var customer stripeCustomer
	if err := json.Unmarshal(data, &customer); err != nil {
		return "", fmt.Errorf("parse stripe customer response: %w", err)
	}

	return customer.ID, nil
}

// CreateSubscription creates a Stripe Subscription for the given customer and price.
// Returns the Stripe subscription ID and status on success.
func (s *StripeService) CreateSubscription(customerID, priceID string) (string, string, error) {
	params := url.Values{}
	params.Set("customer", customerID)
	params.Set("items[0][price]", priceID)
	// payment_behavior=default_incomplete means we get a subscription even before payment
	params.Set("payment_behavior", "default_incomplete")
	params.Set("expand[]", "latest_invoice.payment_intent")

	data, err := s.doRequest(http.MethodPost, "/subscriptions", params)
	if err != nil {
		return "", "", fmt.Errorf("create stripe subscription: %w", err)
	}

	var sub stripeSubscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return "", "", fmt.Errorf("parse stripe subscription response: %w", err)
	}

	return sub.ID, sub.Status, nil
}

// CancelSubscription cancels a Stripe subscription immediately.
func (s *StripeService) CancelSubscription(subscriptionID string) error {
	_, err := s.doRequest(http.MethodPost, "/subscriptions/"+subscriptionID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("cancel stripe subscription: %w", err)
	}
	return nil
}

// VerifyWebhookSignature validates a Stripe webhook signature using HMAC-SHA256.
// payload is the raw request body, sigHeader is the "Stripe-Signature" header value.
func (s *StripeService) VerifyWebhookSignature(payload []byte, sigHeader, webhookSecret string) error {
	// Stripe-Signature header format: t=<timestamp>,v1=<signature>[,v1=<sig2>...]
	var timestamp string
	var signatures []string

	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return fmt.Errorf("invalid stripe-signature header")
	}

	// Compute expected signature: HMAC-SHA256(secret, "<timestamp>.<payload>")
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write([]byte(timestamp + "." + string(payload)))
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return fmt.Errorf("stripe webhook signature mismatch")
}

// HandleWebhookEvent parses and processes a verified Stripe webhook event.
// It updates the StripeCustomer record and enables/disables the VPN client accordingly.
func (s *StripeService) HandleWebhookEvent(payload []byte) error {
	var event stripeWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("parse stripe webhook event: %w", err)
	}

	logger.Infof("Stripe webhook event received: %s (id=%s)", event.Type, event.ID)

	switch event.Type {
	case "customer.subscription.created":
		return s.handleSubscriptionUpdate(event.Data.Object, false)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdate(event.Data.Object, false)
	case "customer.subscription.deleted":
		return s.handleSubscriptionUpdate(event.Data.Object, true)
	case "invoice.paid":
		return s.handleInvoiceEvent(event.Data.Object, true)
	case "invoice.payment_failed":
		return s.handleInvoiceEvent(event.Data.Object, false)
	default:
		// Unhandled event type – log and ignore
		logger.Debugf("Stripe webhook: unhandled event type %s", event.Type)
	}

	return nil
}

// handleSubscriptionUpdate updates the local StripeCustomer record and client state
// based on a subscription object from a webhook event.
func (s *StripeService) handleSubscriptionUpdate(raw json.RawMessage, deleted bool) error {
	var sub stripeSubscription
	if err := json.Unmarshal(raw, &sub); err != nil {
		return err
	}

	db := database.GetDB()
	var record model.StripeCustomer
	if err := db.Where("stripe_subscription_id = ?", sub.ID).First(&record).Error; err != nil {
		logger.Warningf("Stripe webhook: subscription %s not found in local DB", sub.ID)
		return nil
	}

	newStatus := sub.Status
	if deleted {
		newStatus = "canceled"
	}

	record.Status = newStatus
	record.UpdatedAt = time.Now().Unix()
	if err := db.Save(&record).Error; err != nil {
		return fmt.Errorf("update stripe customer record: %w", err)
	}

	enable := newStatus == "active" || newStatus == "trialing"
	return s.setClientEnabled(record.ClientEmail, enable)
}

// handleInvoiceEvent enables or disables a client based on invoice payment outcome.
func (s *StripeService) handleInvoiceEvent(raw json.RawMessage, paid bool) error {
	// Extract the subscription ID from the invoice object
	var invoice struct {
		Subscription string `json:"subscription"`
	}
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return err
	}
	if invoice.Subscription == "" {
		return nil
	}

	db := database.GetDB()
	var record model.StripeCustomer
	if err := db.Where("stripe_subscription_id = ?", invoice.Subscription).First(&record).Error; err != nil {
		logger.Warningf("Stripe webhook: subscription %s not found for invoice event", invoice.Subscription)
		return nil
	}

	status := "inactive"
	if paid {
		status = "active"
	}
	record.Status = status
	record.UpdatedAt = time.Now().Unix()
	if err := db.Save(&record).Error; err != nil {
		return fmt.Errorf("update stripe customer record on invoice: %w", err)
	}

	return s.setClientEnabled(record.ClientEmail, paid)
}

// setClientEnabled enables or disables the VPN client with the given email across all inbounds.
func (s *StripeService) setClientEnabled(email string, enable bool) error {
	var inboundService InboundService
	_, _, err := inboundService.SetClientEnableByEmail(email, enable)
	return err
}

// EnsureStripeCustomer creates (or looks up) a StripeCustomer record for the given
// client email. If Stripe is disabled or no price ID is configured, it is a no-op.
// This is called when a new VPN client is added via AddInboundClient.
func (s *StripeService) EnsureStripeCustomer(email string) {
	enabled, err := s.settingService.GetStripeEnable()
	if err != nil || !enabled {
		return
	}

	priceID, err := s.settingService.GetStripePriceID()
	if err != nil || priceID == "" {
		logger.Warning("Stripe enabled but no price ID configured; skipping customer creation for", email)
		return
	}

	db := database.GetDB()

	// Idempotency: skip if already exists
	var existing model.StripeCustomer
	if err := db.Where("client_email = ?", email).First(&existing).Error; err == nil {
		logger.Debugf("Stripe customer already exists for %s (customer=%s)", email, existing.StripeCustomerID)
		return
	}

	customerID, err := s.CreateCustomer(email)
	if err != nil {
		logger.Errorf("Failed to create Stripe customer for %s: %v", email, err)
		return
	}
	logger.Infof("Created Stripe customer %s for client %s", customerID, email)

	subID, subStatus, err := s.CreateSubscription(customerID, priceID)
	if err != nil {
		logger.Errorf("Failed to create Stripe subscription for customer %s: %v", customerID, err)
		// Still save the customer record even if subscription creation fails
		subID = ""
		subStatus = "incomplete"
	} else {
		logger.Infof("Created Stripe subscription %s (status=%s) for client %s", subID, subStatus, email)
	}

	record := model.StripeCustomer{
		ClientEmail:          email,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subID,
		Status:               subStatus,
		CreatedAt:            time.Now().Unix(),
		UpdatedAt:            time.Now().Unix(),
	}
	if err := db.Create(&record).Error; err != nil {
		logger.Errorf("Failed to save Stripe customer record for %s: %v", email, err)
	}
}
