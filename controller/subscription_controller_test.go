package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func setupSubscriptionControllerTestDB(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.SubscriptionPlan{}); err != nil {
		t.Fatalf("failed to migrate subscription plans: %v", err)
	}
}

func TestAdminUpdateSubscriptionPlanPersistsWindowAmounts(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	confirmPaymentComplianceForTest(t)

	plan := &model.SubscriptionPlan{
		Title:            "Plan",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: model.SubscriptionResetNever,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:            "Plan updated",
			DurationUnit:     model.SubscriptionDurationMonth,
			DurationValue:    1,
			Enabled:          true,
			TotalAmount:      1000,
			FiveHourAmount:   123,
			WeeklyAmount:     456,
			QuotaResetPeriod: model.SubscriptionResetNever,
		},
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/subscription/admin/plans/"+strconv.Itoa(plan.Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var reloaded model.SubscriptionPlan
	if err := model.DB.First(&reloaded, plan.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.FiveHourAmount != 123 || reloaded.WeeklyAmount != 456 {
		t.Fatalf("expected window amounts 123/456, got %d/%d", reloaded.FiveHourAmount, reloaded.WeeklyAmount)
	}
}

func TestAdminCreateSubscriptionPlanRejectsNegativeWindowAmount(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	confirmPaymentComplianceForTest(t)

	body := AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{
			Title:            "Plan",
			DurationUnit:     model.SubscriptionDurationMonth,
			DurationValue:    1,
			Enabled:          true,
			FiveHourAmount:   -1,
			QuotaResetPeriod: model.SubscriptionResetNever,
		},
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/admin/plans", body, 1)

	AdminCreateSubscriptionPlan(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected api error payload with status 200, got %d", recorder.Code)
	}
	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected failure response for negative window amount")
	}
}
