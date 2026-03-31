package controller

import (
	"io"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// StripeController handles Stripe webhook events.
// The webhook endpoint is public (no session auth) but protected by Stripe's
// HMAC-SHA256 signature verification.
type StripeController struct {
	stripeService  service.StripeService
	settingService service.SettingService
}

// NewStripeController creates a new StripeController and registers its routes
// on the provided router group (which should NOT have auth middleware).
func NewStripeController(g *gin.RouterGroup) *StripeController {
	a := &StripeController{}
	a.initRouter(g)
	return a
}

func (a *StripeController) initRouter(g *gin.RouterGroup) {
	g.POST("/webhook", a.handleWebhook)
}

// handleWebhook processes inbound Stripe webhook events.
// It reads the raw body first (required for signature validation), then verifies
// the Stripe-Signature header before handing off to the service layer.
func (a *StripeController) handleWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Warning("Stripe webhook: failed to read body:", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Only verify signature when a webhook secret is configured
	webhookSecret, err := a.settingService.GetStripeWebhookSecret()
	if err != nil {
		logger.Warning("Stripe webhook: failed to read webhook secret:", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if webhookSecret != "" {
		sigHeader := c.GetHeader("Stripe-Signature")
		if sigHeader == "" {
			logger.Warning("Stripe webhook: missing Stripe-Signature header")
			c.Status(http.StatusBadRequest)
			return
		}
		if err := a.stripeService.VerifyWebhookSignature(payload, sigHeader, webhookSecret); err != nil {
			logger.Warning("Stripe webhook: signature verification failed:", err)
			c.Status(http.StatusUnauthorized)
			return
		}
	}

	if err := a.stripeService.HandleWebhookEvent(payload); err != nil {
		logger.Error("Stripe webhook: event handling error:", err)
		// Return 200 to Stripe anyway so it does not retry on internal errors
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)
}
