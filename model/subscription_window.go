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
