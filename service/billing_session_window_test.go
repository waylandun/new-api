package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestNewSubscriptionWindowAPIErrorIncludesStructuredBlock(t *testing.T) {
	apiErr := newSubscriptionWindowAPIError(0, &model.SubscriptionWindowError{
		Kind:      model.WindowKindFiveHour,
		Limit:     100,
		Used:      100,
		Remaining: 0,
		Required:  1,
		NextReset: 1234567890,
	})

	if apiErr.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", apiErr.StatusCode)
	}
	if apiErr.GetErrorCode() != types.ErrorCodeInsufficientUserQuota {
		t.Fatalf("expected insufficient quota error code, got %s", apiErr.GetErrorCode())
	}

	openAIError := apiErr.ToOpenAIError()
	if openAIError.Type != "subscription_window_exceeded" {
		t.Fatalf("expected subscription_window_exceeded type, got %q", openAIError.Type)
	}
	if openAIError.SubscriptionBlock == nil {
		t.Fatalf("expected subscription block")
	}
	if openAIError.SubscriptionBlock.Kind != "five_hour" ||
		openAIError.SubscriptionBlock.Limit != 100 ||
		openAIError.SubscriptionBlock.Used != 100 ||
		openAIError.SubscriptionBlock.Remaining != 0 ||
		openAIError.SubscriptionBlock.Required != 1 ||
		openAIError.SubscriptionBlock.NextReset != 1234567890 {
		t.Fatalf("unexpected subscription block: %+v", openAIError.SubscriptionBlock)
	}
}

func TestNewBillingSessionSubscriptionFirstKeepsSubscriptionErrorWhenWalletFallbackAlsoInsufficient(t *testing.T) {
	truncate(t)

	const userId = 9101
	seedUser(t, userId, 0)

	plan := &model.SubscriptionPlan{
		Title:            "T",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: model.SubscriptionResetNever,
		Enabled:          true,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	now := time.Now().Unix()
	sub := &model.UserSubscription{
		UserId:      userId,
		PlanId:      plan.Id,
		AmountTotal: 100,
		AmountUsed:  92,
		StartTime:   now,
		EndTime:     now + 3600,
		Status:      "active",
	}
	if err := model.DB.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:          userId,
		IsPlayground:    true,
		RequestId:       "req-subscription-first-preserve-error",
		OriginModelName: "m",
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(c, info, 9)
	if apiErr == nil {
		t.Fatalf("expected subscription quota error, got session=%v", session)
	}

	openAIError := apiErr.ToOpenAIError()
	if openAIError.Type != "subscription_window_exceeded" {
		t.Fatalf("expected subscription error to be preserved, got type=%q message=%q", openAIError.Type, openAIError.Message)
	}
	if openAIError.SubscriptionBlock == nil {
		t.Fatalf("expected subscription block")
	}
	if openAIError.SubscriptionBlock.Kind != string(model.WindowKindTotal) ||
		openAIError.SubscriptionBlock.Limit != 100 ||
		openAIError.SubscriptionBlock.Used != 92 ||
		openAIError.SubscriptionBlock.Remaining != 8 ||
		openAIError.SubscriptionBlock.Required != 9 {
		t.Fatalf("unexpected subscription block: %+v", openAIError.SubscriptionBlock)
	}
}
