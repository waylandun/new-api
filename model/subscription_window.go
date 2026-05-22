package model

import (
	"errors"
	"fmt"
)

// WindowKind 标识哪一层窗口
type WindowKind string

const (
	WindowKindTotal    WindowKind = "total"
	WindowKindWeekly   WindowKind = "weekly"
	WindowKindFiveHour WindowKind = "five_hour"
)

const (
	fiveHourWindowSeconds int64 = 5 * 60 * 60
	weeklyWindowSeconds   int64 = 7 * 24 * 60 * 60
)

// WindowEvalResult 是 EvaluateConsume 的纯函数返回，不修改 sub
type WindowEvalResult struct {
	Allow     bool
	BlockedBy WindowKind   // Allow=false 时填
	NextReset int64        // BlockedBy 对应窗口的下一次重置时刻；total 层填 0
	Resets    []WindowKind // 本次评估中需要"懒重置"的窗口列表
}

// ErrSubscriptionWindowExceeded 是 SubscriptionWindowError 的 sentinel
var ErrSubscriptionWindowExceeded = errors.New("subscription window exceeded")

type SubscriptionWindowError struct {
	Kind      WindowKind
	Limit     int64
	Used      int64
	NextReset int64 // unix；0 表示无自动重置（如 total 层）
}

func (e *SubscriptionWindowError) Error() string {
	return fmt.Sprintf("subscription window exceeded: kind=%s, used=%d/%d, next_reset=%d",
		e.Kind, e.Used, e.Limit, e.NextReset)
}

func (e *SubscriptionWindowError) Is(target error) bool {
	return target == ErrSubscriptionWindowExceeded
}

const maxInt64 int64 = 1<<63 - 1

// remaining returns the quota left in a layer.
// Returns maxInt64 when limit == 0 (layer disabled / unlimited convention).
func remaining(limit, used int64) int64 {
	if limit == 0 {
		return maxInt64
	}
	if used >= limit {
		return 0
	}
	return limit - used
}

// windowExpired reports whether a rolling window should be reset.
// Returns true if the window has never been opened (start == 0) or has expired (now >= start+size).
// Callers must check limit > 0 separately to decide whether the layer is enabled.
func windowExpired(start, size, now int64) bool {
	return start == 0 || now >= start+size
}

// EvaluateConsume 检测三层窗口能否容纳 amount，不修改 sub。
// 优先级：total < weekly < fiveHour（数字小=越严重，先报）。
func EvaluateConsume(sub *UserSubscription, plan *SubscriptionPlan, amount, now int64) WindowEvalResult {
	res := WindowEvalResult{Allow: true}

	fhUsedEffective := sub.FiveHourUsed
	if sub.FiveHourLimit > 0 && windowExpired(sub.FiveHourWindowStart, fiveHourWindowSeconds, now) {
		res.Resets = append(res.Resets, WindowKindFiveHour)
		fhUsedEffective = 0
	}
	wkUsedEffective := sub.WeeklyUsed
	if sub.WeeklyLimit > 0 && windowExpired(sub.WeeklyWindowStart, weeklyWindowSeconds, now) {
		res.Resets = append(res.Resets, WindowKindWeekly)
		wkUsedEffective = 0
	}

	if remaining(sub.AmountTotal, sub.AmountUsed) < amount {
		return WindowEvalResult{
			Allow: false, BlockedBy: WindowKindTotal, NextReset: 0,
		}
	}
	if sub.WeeklyLimit > 0 && remaining(sub.WeeklyLimit, wkUsedEffective) < amount {
		nextReset := now + weeklyWindowSeconds
		if !windowExpired(sub.WeeklyWindowStart, weeklyWindowSeconds, now) {
			nextReset = sub.WeeklyWindowStart + weeklyWindowSeconds
		}
		return WindowEvalResult{
			Allow: false, BlockedBy: WindowKindWeekly, NextReset: nextReset,
		}
	}
	if sub.FiveHourLimit > 0 && remaining(sub.FiveHourLimit, fhUsedEffective) < amount {
		nextReset := now + fiveHourWindowSeconds
		if !windowExpired(sub.FiveHourWindowStart, fiveHourWindowSeconds, now) {
			nextReset = sub.FiveHourWindowStart + fiveHourWindowSeconds
		}
		return WindowEvalResult{
			Allow: false, BlockedBy: WindowKindFiveHour, NextReset: nextReset,
		}
	}

	return res
}

// ApplyConsume 重置过期窗口并扣减；调用方需保证 result.Allow=true。
func ApplyConsume(sub *UserSubscription, result WindowEvalResult, amount, now int64) {
	for _, k := range result.Resets {
		switch k {
		case WindowKindFiveHour:
			sub.FiveHourUsed = 0
			sub.FiveHourWindowStart = now
		case WindowKindWeekly:
			sub.WeeklyUsed = 0
			sub.WeeklyWindowStart = now
		}
	}
	sub.AmountUsed += amount
	if sub.FiveHourLimit > 0 {
		sub.FiveHourUsed += amount
	}
	if sub.WeeklyLimit > 0 {
		sub.WeeklyUsed += amount
	}
}

// ApplyDelta：事后调整（Settle 补扣 / 退款 / 全量 refund）。
// 不重新开窗、不重置；窗口已过期或未开窗的层不动。
func ApplyDelta(sub *UserSubscription, delta, now int64) {
	if delta == 0 {
		return
	}
	clampAdd := func(cur, d int64) int64 {
		v := cur + d
		if v < 0 {
			return 0
		}
		return v
	}
	sub.AmountUsed = clampAdd(sub.AmountUsed, delta)
	if sub.FiveHourLimit > 0 && sub.FiveHourWindowStart > 0 &&
		!windowExpired(sub.FiveHourWindowStart, fiveHourWindowSeconds, now) {
		sub.FiveHourUsed = clampAdd(sub.FiveHourUsed, delta)
	}
	if sub.WeeklyLimit > 0 && sub.WeeklyWindowStart > 0 &&
		!windowExpired(sub.WeeklyWindowStart, weeklyWindowSeconds, now) {
		sub.WeeklyUsed = clampAdd(sub.WeeklyUsed, delta)
	}
}
