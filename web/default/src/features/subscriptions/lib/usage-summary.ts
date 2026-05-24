/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { UserSubscriptionRecord } from '../types'

const FIVE_HOUR_SECONDS = 5 * 60 * 60
const WEEK_SECONDS = 7 * 24 * 60 * 60

export const selfSubscriptionQueryKey = ['subscription', 'self'] as const
export const publicSubscriptionPlansQueryKey = [
  'subscription',
  'public-plans',
] as const

export type SubscriptionUsageRowKey = 'plan' | 'five_hour' | 'weekly'
export type SubscriptionUsageLabelKey =
  | 'Plan quota'
  | '5-hour limit'
  | 'Weekly limit'

export interface SubscriptionUsageRow {
  key: SubscriptionUsageRowKey
  labelKey: SubscriptionUsageLabelKey
  isUnlimited: boolean
  isRequestAvailable: boolean
  actualRemaining: number | null
  actualRemainingPercent: number | null
  minimumRequestQuota: number
  remainingPercent: number | null
  progressValue: number
  resetAt?: number
}

export interface SubscriptionUsageSummary {
  id: number
  planId: number
  title?: string
  endTime: number
  rows: SubscriptionUsageRow[]
  lowestRemainingPercent: number | null
}

function normalizeNumber(value: unknown): number {
  const num = Number(value || 0)
  return Number.isFinite(num) ? num : 0
}

function clampPercent(value: number): number {
  return Math.min(100, Math.max(0, Math.round(value)))
}

function getEffectiveRollingWindow(
  used: unknown,
  windowStart: number,
  windowSeconds: number,
  nowSeconds: number
): { used: number; resetAt?: number } {
  if (windowStart <= 0) {
    return { used: 0 }
  }

  const resetAt = windowStart + windowSeconds
  if (nowSeconds >= resetAt) {
    return { used: 0 }
  }

  return {
    used: normalizeNumber(used),
    resetAt,
  }
}

function getEffectivePlanUsed(
  used: unknown,
  nextResetTime: number,
  nowSeconds: number
): number {
  if (nextResetTime > 0 && nowSeconds >= nextResetTime) {
    return 0
  }
  return normalizeNumber(used)
}

function getEffectiveMinimumRequestQuota(minimumRequestQuota: number): number {
  return Math.max(1, normalizeNumber(minimumRequestQuota))
}

function buildUsageRow(
  key: SubscriptionUsageRowKey,
  labelKey: SubscriptionUsageLabelKey,
  used: number,
  limit: number,
  minimumRequestQuota: number,
  resetAt?: number
): SubscriptionUsageRow {
  if (limit <= 0) {
    return {
      key,
      labelKey,
      isUnlimited: true,
      isRequestAvailable: true,
      actualRemaining: null,
      actualRemainingPercent: null,
      minimumRequestQuota,
      remainingPercent: null,
      progressValue: 100,
      resetAt,
    }
  }

  const remaining = Math.max(0, limit - Math.max(0, used))
  const actualRemainingPercent = clampPercent((remaining / limit) * 100)
  const isRequestAvailable =
    minimumRequestQuota <= 0 || remaining >= minimumRequestQuota
  const remainingPercent = isRequestAvailable ? actualRemainingPercent : 0

  return {
    key,
    labelKey,
    isUnlimited: false,
    isRequestAvailable,
    actualRemaining: remaining,
    actualRemainingPercent,
    minimumRequestQuota,
    remainingPercent,
    progressValue: remainingPercent,
    resetAt,
  }
}

export function isActiveSubscriptionRecord(
  record: UserSubscriptionRecord | null | undefined,
  nowSeconds = Date.now() / 1000
): boolean {
  const subscription = record?.subscription
  if (!subscription) return false
  return subscription.status === 'active' && subscription.end_time > nowSeconds
}

export function getLowestRemainingPercent(
  summaries: SubscriptionUsageSummary[]
): number | null {
  const values = summaries.flatMap((summary) =>
    summary.rows
      .map((row) => row.remainingPercent)
      .filter((value): value is number => value !== null)
  )
  if (values.length === 0) return null
  return Math.min(...values)
}

export function buildSubscriptionUsageSummary(
  record: UserSubscriptionRecord | null | undefined,
  title?: string,
  nowSeconds = Date.now() / 1000,
  minimumRequestQuota = 0
): SubscriptionUsageSummary | null {
  if (!isActiveSubscriptionRecord(record, nowSeconds)) return null

  const subscription = record?.subscription
  if (!subscription) return null
  const effectiveMinimumRequestQuota =
    getEffectiveMinimumRequestQuota(minimumRequestQuota)

  const fiveHourWindowStart = normalizeNumber(
    subscription.five_hour_window_start
  )
  const weeklyWindowStart = normalizeNumber(subscription.weekly_window_start)
  const nextResetTime = normalizeNumber(subscription.next_reset_time)
  const planUsed = getEffectivePlanUsed(
    subscription.amount_used,
    nextResetTime,
    nowSeconds
  )
  const fiveHourWindow = getEffectiveRollingWindow(
    subscription.five_hour_used,
    fiveHourWindowStart,
    FIVE_HOUR_SECONDS,
    nowSeconds
  )
  const weeklyWindow = getEffectiveRollingWindow(
    subscription.weekly_used,
    weeklyWindowStart,
    WEEK_SECONDS,
    nowSeconds
  )

  const rows: SubscriptionUsageRow[] = [
    buildUsageRow(
      'plan',
      'Plan quota',
      planUsed,
      normalizeNumber(subscription.amount_total),
      effectiveMinimumRequestQuota,
      nextResetTime > nowSeconds ? nextResetTime : undefined
    ),
    buildUsageRow(
      'five_hour',
      '5-hour limit',
      fiveHourWindow.used,
      normalizeNumber(subscription.five_hour_limit),
      effectiveMinimumRequestQuota,
      fiveHourWindow.resetAt
    ),
    buildUsageRow(
      'weekly',
      'Weekly limit',
      weeklyWindow.used,
      normalizeNumber(subscription.weekly_limit),
      effectiveMinimumRequestQuota,
      weeklyWindow.resetAt
    ),
  ]

  return {
    id: subscription.id,
    planId: subscription.plan_id,
    title,
    endTime: subscription.end_time,
    rows,
    lowestRemainingPercent: getLowestRemainingPercent([
      {
        id: subscription.id,
        planId: subscription.plan_id,
        title,
        endTime: subscription.end_time,
        rows,
        lowestRemainingPercent: null,
      },
    ]),
  }
}

export function buildSubscriptionUsageSummaries(
  records: UserSubscriptionRecord[] = [],
  planTitleMap?: Map<number, string>,
  nowSeconds = Date.now() / 1000,
  minimumRequestQuota = 0
): SubscriptionUsageSummary[] {
  return records
    .map((record) =>
      buildSubscriptionUsageSummary(
        record,
        planTitleMap?.get(record?.subscription?.plan_id || 0),
        nowSeconds,
        minimumRequestQuota
      )
    )
    .filter((summary): summary is SubscriptionUsageSummary => summary !== null)
}
