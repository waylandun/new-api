package model

import (
	"errors"
	"strings"
	"testing"
)

func mkPlanForPreconsumeTest(t *testing.T, total, fh, wk int64) *SubscriptionPlan {
	t.Helper()
	p := &SubscriptionPlan{
		Title:            "T",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      total,
		FiveHourAmount:   fh,
		WeeklyAmount:     wk,
		QuotaResetPeriod: SubscriptionResetNever,
		Enabled:          true,
	}
	if err := DB.Create(p).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return p
}

func mkActiveSubForPreconsumeTest(t *testing.T, userId int, p *SubscriptionPlan, endTime int64) *UserSubscription {
	t.Helper()
	sub := &UserSubscription{
		UserId:        userId,
		PlanId:        p.Id,
		AmountTotal:   p.TotalAmount,
		FiveHourLimit: p.FiveHourAmount,
		WeeklyLimit:   p.WeeklyAmount,
		StartTime:     GetDBTimestamp(),
		EndTime:       endTime,
		Status:        "active",
	}
	if err := DB.Create(sub).Error; err != nil {
		t.Fatalf("create sub: %v", err)
	}
	return sub
}

func TestPreConsume_NewSub_OpensFiveHourAndWeekly(t *testing.T) {
	truncateTables(t)
	p := mkPlanForPreconsumeTest(t, 0, 1000, 5000)
	mkActiveSubForPreconsumeTest(t, 1001, p, GetDBTimestamp()+86400)

	res, err := PreConsumeUserSubscription("req-pc-1", 1001, "m", 0, 100)
	if err != nil {
		t.Fatalf("pre-consume: %v", err)
	}
	var sub UserSubscription
	if err := DB.First(&sub, res.UserSubscriptionId).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if sub.FiveHourWindowStart == 0 || sub.WeeklyWindowStart == 0 {
		t.Fatalf("expected windows opened, got %+v", sub)
	}
	if sub.FiveHourUsed != 100 || sub.WeeklyUsed != 100 {
		t.Fatalf("expected used=100/100, got %d/%d", sub.FiveHourUsed, sub.WeeklyUsed)
	}
}

func TestPreConsume_BlockedByFiveHour_ReturnsWindowError(t *testing.T) {
	truncateTables(t)
	p := mkPlanForPreconsumeTest(t, 0, 1000, 0)
	sub := mkActiveSubForPreconsumeTest(t, 1002, p, GetDBTimestamp()+86400)
	sub.FiveHourUsed = 1000
	sub.FiveHourWindowStart = GetDBTimestamp()
	if err := DB.Save(sub).Error; err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := PreConsumeUserSubscription("req-pc-2", 1002, "m", 0, 1)
	var winErr *SubscriptionWindowError
	if !errors.As(err, &winErr) {
		t.Fatalf("expected SubscriptionWindowError, got %v", err)
	}
	if winErr.Kind != WindowKindFiveHour || winErr.Limit != 1000 || winErr.Used != 1000 {
		t.Fatalf("unexpected fields: %+v", winErr)
	}
	if winErr.Remaining != 0 || winErr.Required != 1 {
		t.Fatalf("unexpected remaining/required: %+v", winErr)
	}
	if !errors.Is(err, ErrSubscriptionWindowExceeded) {
		t.Fatalf("errors.Is should match sentinel")
	}
}

func TestPreConsume_BlockedSubFallbackToNext(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	pBlocked := mkPlanForPreconsumeTest(t, 0, 100, 0)
	pOpen := mkPlanForPreconsumeTest(t, 0, 1000, 0)

	subBlocked := mkActiveSubForPreconsumeTest(t, 1003, pBlocked, now+1000)
	subBlocked.FiveHourUsed = 100
	subBlocked.FiveHourWindowStart = now
	if err := DB.Save(subBlocked).Error; err != nil {
		t.Fatalf("save: %v", err)
	}
	mkActiveSubForPreconsumeTest(t, 1003, pOpen, now+2000)

	res, err := PreConsumeUserSubscription("req-pc-3", 1003, "m", 0, 50)
	if err != nil {
		t.Fatalf("expected success via fallback, got %v", err)
	}
	if res.UserSubscriptionId == subBlocked.Id {
		t.Fatalf("expected to fall back to second sub, used blocked one")
	}
}

func TestPreConsume_RefundDoesNotResetWindow(t *testing.T) {
	truncateTables(t)
	p := mkPlanForPreconsumeTest(t, 0, 1000, 0)
	mkActiveSubForPreconsumeTest(t, 1004, p, GetDBTimestamp()+86400)

	if _, err := PreConsumeUserSubscription("req-pc-4", 1004, "m", 0, 200); err != nil {
		t.Fatalf("preconsume: %v", err)
	}
	var subBefore UserSubscription
	if err := DB.Where("user_id = ?", 1004).First(&subBefore).Error; err != nil {
		t.Fatalf("reload before: %v", err)
	}
	startBefore := subBefore.FiveHourWindowStart

	if err := RefundSubscriptionPreConsume("req-pc-4"); err != nil {
		t.Fatalf("refund: %v", err)
	}
	var subAfter UserSubscription
	if err := DB.Where("user_id = ?", 1004).First(&subAfter).Error; err != nil {
		t.Fatalf("reload after: %v", err)
	}
	if subAfter.FiveHourWindowStart != startBefore {
		t.Fatalf("refund should not move window: before=%d after=%d", startBefore, subAfter.FiveHourWindowStart)
	}
	if subAfter.FiveHourUsed != 0 {
		t.Fatalf("expected fiveHourUsed back to 0 after refund, got %d", subAfter.FiveHourUsed)
	}
}

func TestPreConsume_Idempotent_SameRequestId(t *testing.T) {
	truncateTables(t)
	p := mkPlanForPreconsumeTest(t, 0, 1000, 0)
	mkActiveSubForPreconsumeTest(t, 1005, p, GetDBTimestamp()+86400)

	if _, err := PreConsumeUserSubscription("req-pc-5", 1005, "m", 0, 100); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := PreConsumeUserSubscription("req-pc-5", 1005, "m", 0, 100); err != nil {
		t.Fatalf("second (idempotent): %v", err)
	}
	var sub UserSubscription
	if err := DB.Where("user_id = ?", 1005).First(&sub).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if sub.FiveHourUsed != 100 {
		t.Fatalf("expected used=100 after idempotent retry, got %d", sub.FiveHourUsed)
	}
}

func TestPreConsume_LegacySub_ZeroLimits_BehavesAsBefore(t *testing.T) {
	truncateTables(t)
	p := mkPlanForPreconsumeTest(t, 1000, 0, 0)
	mkActiveSubForPreconsumeTest(t, 1006, p, GetDBTimestamp()+86400)

	if _, err := PreConsumeUserSubscription("req-pc-6", 1006, "m", 0, 500); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := PreConsumeUserSubscription("req-pc-7", 1006, "m", 0, 600); err == nil {
		t.Fatalf("expected total exhaustion error, got nil")
	}
}

func TestPreConsume_MultipleBlockedSubs_ReturnsMostSevereWindowError(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()

	pTotal := mkPlanForPreconsumeTest(t, 100, 0, 0)
	totalSub := mkActiveSubForPreconsumeTest(t, 1007, pTotal, now+1000)
	totalSub.AmountUsed = 100
	if err := DB.Save(totalSub).Error; err != nil {
		t.Fatalf("save total sub: %v", err)
	}

	pFiveHour := mkPlanForPreconsumeTest(t, 0, 100, 0)
	fiveHourSub := mkActiveSubForPreconsumeTest(t, 1007, pFiveHour, now+2000)
	fiveHourSub.FiveHourUsed = 100
	fiveHourSub.FiveHourWindowStart = now
	if err := DB.Save(fiveHourSub).Error; err != nil {
		t.Fatalf("save five-hour sub: %v", err)
	}

	_, err := PreConsumeUserSubscription("req-pc-8", 1007, "m", 0, 1)
	var winErr *SubscriptionWindowError
	if !errors.As(err, &winErr) {
		t.Fatalf("expected SubscriptionWindowError, got %v", err)
	}
	if winErr.Kind != WindowKindTotal {
		t.Fatalf("expected total to be preferred over later five-hour block, got %+v", winErr)
	}
}

func TestPreConsume_WindowExceededRecordsManageLog(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	p := mkPlanForPreconsumeTest(t, 0, 100, 0)
	sub := mkActiveSubForPreconsumeTest(t, 1008, p, now+86400)
	sub.FiveHourUsed = 100
	sub.FiveHourWindowStart = now
	if err := DB.Save(sub).Error; err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := PreConsumeUserSubscription("req-pc-9", 1008, "m", 0, 1)
	if err == nil {
		t.Fatalf("expected window error")
	}

	var logs []Log
	if err := LOG_DB.Where("user_id = ? AND type = ?", 1008, LogTypeManage).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one manage log, got %d", len(logs))
	}
	if logs[0].Content == "" || !strings.Contains(logs[0].Content, "five_hour") {
		t.Fatalf("unexpected log content: %q", logs[0].Content)
	}
}

func TestAdminResetUserSubscriptionWindow_RecordsManageLog(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	p := mkPlanForPreconsumeTest(t, 0, 100, 0)
	sub := mkActiveSubForPreconsumeTest(t, 1009, p, now+86400)
	sub.FiveHourUsed = 100
	sub.FiveHourWindowStart = now
	if err := DB.Save(sub).Error; err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := AdminResetUserSubscriptionWindow(sub.Id, WindowKindFiveHour); err != nil {
		t.Fatalf("reset: %v", err)
	}

	var reloaded UserSubscription
	if err := DB.First(&reloaded, sub.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.FiveHourUsed != 0 || reloaded.FiveHourWindowStart != 0 {
		t.Fatalf("expected window reset, got used=%d start=%d", reloaded.FiveHourUsed, reloaded.FiveHourWindowStart)
	}

	var logs []Log
	if err := LOG_DB.Where("user_id = ? AND type = ?", 1009, LogTypeManage).Find(&logs).Error; err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one manage log, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Content, "five_hour") {
		t.Fatalf("unexpected log content: %q", logs[0].Content)
	}
}
