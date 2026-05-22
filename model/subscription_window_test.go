package model

import (
	"errors"
	"testing"
)

func subWith(used, total, fhLimit, fhUsed, fhStart, wkLimit, wkUsed, wkStart int64) *UserSubscription {
	return &UserSubscription{
		AmountTotal: total, AmountUsed: used,
		FiveHourLimit: fhLimit, FiveHourUsed: fhUsed, FiveHourWindowStart: fhStart,
		WeeklyLimit: wkLimit, WeeklyUsed: wkUsed, WeeklyWindowStart: wkStart,
	}
}

func TestEvaluate_AllLimitsZero_AllowsAlways(t *testing.T) {
	sub := subWith(1<<40, 0, 0, 0, 0, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 1<<30, 1_000_000_000)
	if !res.Allow || res.BlockedBy != "" || len(res.Resets) != 0 {
		t.Fatalf("want unconditional allow, got %+v", res)
	}
}

func TestEvaluate_TotalExhausted_BlocksTotal(t *testing.T) {
	sub := subWith(100, 100, 0, 0, 0, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 1, 1_000_000_000)
	if res.Allow || res.BlockedBy != WindowKindTotal || res.NextReset != 0 {
		t.Fatalf("want blocked by total, got %+v", res)
	}
}

func TestEvaluate_FiveHourFresh_OpensWindow(t *testing.T) {
	sub := subWith(0, 0, 1000, 0, 0, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 100, 1_000_000_000)
	if !res.Allow || len(res.Resets) != 1 || res.Resets[0] != WindowKindFiveHour {
		t.Fatalf("want allow + reset[fiveHour], got %+v", res)
	}
}

func TestEvaluate_FiveHourExpired_AutoResets(t *testing.T) {
	now := int64(1_000_000_000)
	start := now - fiveHourWindowSeconds - 1
	sub := subWith(0, 0, 1000, 999, start, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 100, now)
	if !res.Allow || len(res.Resets) != 1 || res.Resets[0] != WindowKindFiveHour {
		t.Fatalf("want allow + reset[fiveHour], got %+v", res)
	}
}

func TestEvaluate_FiveHourActive_Blocks(t *testing.T) {
	now := int64(1_000_000_000)
	start := now - 100
	sub := subWith(0, 0, 1000, 1000, start, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 1, now)
	if res.Allow || res.BlockedBy != WindowKindFiveHour {
		t.Fatalf("want blocked by fiveHour, got %+v", res)
	}
	if res.NextReset != start+fiveHourWindowSeconds {
		t.Fatalf("want NextReset=%d, got %d", start+fiveHourWindowSeconds, res.NextReset)
	}
}

func TestEvaluate_WeeklyAndFiveHourBothBlock_PrefersWeekly(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(0, 0, 1000, 1000, now-100, 5000, 5000, now-200)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 1, now)
	if res.Allow || res.BlockedBy != WindowKindWeekly {
		t.Fatalf("want blocked by weekly (priority), got %+v", res)
	}
}

func TestEvaluate_AllThreeBlock_PrefersTotal(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(100, 100, 1000, 1000, now-100, 5000, 5000, now-200)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 1, now)
	if res.Allow || res.BlockedBy != WindowKindTotal {
		t.Fatalf("want blocked by total (priority), got %+v", res)
	}
}

func TestEvaluate_AmountExceedsFiveHourLimit(t *testing.T) {
	sub := subWith(0, 0, 100, 0, 0, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 200, 1_000_000_000)
	if res.Allow || res.BlockedBy != WindowKindFiveHour {
		t.Fatalf("want blocked by fiveHour, got %+v", res)
	}
}

func TestApply_ResetsThenIncrements(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(0, 0, 1000, 800, now-fiveHourWindowSeconds-1, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 100, now)
	if !res.Allow {
		t.Fatalf("want allow, got %+v", res)
	}
	ApplyConsume(sub, res, 100, now)
	if sub.FiveHourUsed != 100 || sub.FiveHourWindowStart != now {
		t.Fatalf("want fiveHourUsed=100 windowStart=%d, got used=%d start=%d",
			now, sub.FiveHourUsed, sub.FiveHourWindowStart)
	}
}

func TestApply_DisabledLayerNotTouched(t *testing.T) {
	sub := subWith(0, 0, 0, 0, 0, 0, 0, 0)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 100, 1_000_000_000)
	ApplyConsume(sub, res, 100, 1_000_000_000)
	if sub.FiveHourUsed != 0 || sub.WeeklyUsed != 0 {
		t.Fatalf("disabled layers must stay 0, got fh=%d wk=%d", sub.FiveHourUsed, sub.WeeklyUsed)
	}
	if sub.AmountUsed != 100 {
		t.Fatalf("AmountUsed must increment, got %d", sub.AmountUsed)
	}
}

func TestApplyDelta_WithinWindow_Decreases(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(500, 0, 1000, 500, now-100, 0, 0, 0)
	ApplyDelta(sub, -200, now)
	if sub.AmountUsed != 300 || sub.FiveHourUsed != 300 {
		t.Fatalf("want 300/300, got %d/%d", sub.AmountUsed, sub.FiveHourUsed)
	}
}

func TestApplyDelta_ExpiredWindow_NoOpForLayer(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(500, 0, 1000, 500, now-fiveHourWindowSeconds-1, 0, 0, 0)
	ApplyDelta(sub, -200, now)
	if sub.AmountUsed != 300 {
		t.Fatalf("AmountUsed should adjust regardless, got %d", sub.AmountUsed)
	}
	if sub.FiveHourUsed != 500 {
		t.Fatalf("expired five-hour layer must not change, got %d", sub.FiveHourUsed)
	}
}

func TestApplyDelta_ClampsToZero(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(50, 0, 1000, 50, now-100, 0, 0, 0)
	ApplyDelta(sub, -1000, now)
	if sub.AmountUsed != 0 || sub.FiveHourUsed != 0 {
		t.Fatalf("want clamped to 0, got %d / %d", sub.AmountUsed, sub.FiveHourUsed)
	}
}

func TestSubscriptionWindowError_IsErrSubscriptionWindowExceeded(t *testing.T) {
	e := &SubscriptionWindowError{Kind: WindowKindFiveHour}
	if !errors.Is(e, ErrSubscriptionWindowExceeded) {
		t.Fatalf("errors.Is should match sentinel")
	}
}

func TestApplyDelta_ZeroDelta_NoOp(t *testing.T) {
	now := int64(1_000_000_000)
	sub := subWith(500, 0, 1000, 500, now-100, 5000, 500, now-100)
	ApplyDelta(sub, 0, now)
	if sub.AmountUsed != 500 || sub.FiveHourUsed != 500 || sub.WeeklyUsed != 500 {
		t.Fatalf("zero delta must not modify any used counter, got total=%d fh=%d wk=%d",
			sub.AmountUsed, sub.FiveHourUsed, sub.WeeklyUsed)
	}
}

func TestApply_ResetsBothWindowsAtOnce(t *testing.T) {
	now := int64(1_000_000_000)
	// Both windows expired (started before their respective sizes ago).
	sub := subWith(0, 0,
		1000, 800, now-fiveHourWindowSeconds-1,
		5000, 4000, now-weeklyWindowSeconds-1,
	)
	res := EvaluateConsume(sub, &SubscriptionPlan{}, 100, now)
	if !res.Allow {
		t.Fatalf("want allow, got %+v", res)
	}
	if len(res.Resets) != 2 {
		t.Fatalf("want 2 resets, got %d (%+v)", len(res.Resets), res.Resets)
	}
	ApplyConsume(sub, res, 100, now)
	if sub.FiveHourUsed != 100 || sub.FiveHourWindowStart != now {
		t.Fatalf("fiveHour: want 100/now, got %d/%d", sub.FiveHourUsed, sub.FiveHourWindowStart)
	}
	if sub.WeeklyUsed != 100 || sub.WeeklyWindowStart != now {
		t.Fatalf("weekly: want 100/now, got %d/%d", sub.WeeklyUsed, sub.WeeklyWindowStart)
	}
}
