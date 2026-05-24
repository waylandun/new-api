/*
Copyright (C) 2025 QuantumNous

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

const FIVE_HOUR_SECONDS = 5 * 60 * 60;
const WEEK_SECONDS = 7 * 24 * 60 * 60;

const LABEL_KEYS = {
  plan: '套餐额度',
  fiveHour: '5 小时限额',
  weekly: '周限额',
};

function normalizeNumber(value) {
  const num = Number(value || 0);
  return Number.isFinite(num) ? num : 0;
}

function clampPercent(value) {
  return Math.min(100, Math.max(0, Math.round(value)));
}

function getConfiguredPreConsumedQuota() {
  if (typeof localStorage === 'undefined') return 500;

  const directStoredValue = localStorage.getItem('pre_consumed_quota');
  if (directStoredValue !== null && directStoredValue !== undefined) {
    const directValue = normalizeNumber(directStoredValue);
    if (directValue > 0 || directStoredValue === '0') {
      return Math.max(1, directValue);
    }
  }

  try {
    const status = JSON.parse(localStorage.getItem('status') || '{}');
    if (
      status?.pre_consumed_quota !== null &&
      status?.pre_consumed_quota !== undefined
    ) {
      return Math.max(1, normalizeNumber(status.pre_consumed_quota));
    }
    return 500;
  } catch {
    return 500;
  }
}

function getEffectiveRollingWindow(used, windowStart, windowSeconds, nowSeconds) {
  if (windowStart <= 0) {
    return { used: 0 };
  }

  const resetAt = windowStart + windowSeconds;
  if (nowSeconds >= resetAt) {
    return { used: 0 };
  }

  return {
    used: normalizeNumber(used),
    resetAt,
  };
}

function getEffectivePlanUsed(used, nextResetTime, nowSeconds) {
  if (nextResetTime > 0 && nowSeconds >= nextResetTime) {
    return 0;
  }
  return normalizeNumber(used);
}

function buildUsageRow(key, used, limit, minimumRequestQuota, resetAt) {
  if (limit <= 0) {
    return {
      key,
      labelKey: LABEL_KEYS[key],
      isUnlimited: true,
      isRequestAvailable: true,
      actualRemaining: null,
      minimumRequestQuota,
      remainingPercent: null,
      progressValue: 100,
      resetAt,
    };
  }

  const remaining = Math.max(0, limit - Math.max(0, used));
  const actualRemainingPercent = clampPercent((remaining / limit) * 100);
  const isRequestAvailable =
    minimumRequestQuota <= 0 || remaining >= minimumRequestQuota;
  const remainingPercent = isRequestAvailable ? actualRemainingPercent : 0;

  return {
    key,
    labelKey: LABEL_KEYS[key],
    isUnlimited: false,
    isRequestAvailable,
    actualRemaining: remaining,
    minimumRequestQuota,
    remainingPercent,
    progressValue: remainingPercent,
    resetAt,
  };
}

export function isActiveSubscriptionRecord(record, nowSeconds = Date.now() / 1000) {
  const subscription = record?.subscription;
  if (!subscription) return false;
  return subscription.status === 'active' && subscription.end_time > nowSeconds;
}

export function getLowestRemainingPercent(summaries = []) {
  const values = summaries.flatMap((summary) =>
    (summary.rows || [])
      .map((row) => row.remainingPercent)
      .filter((value) => value !== null && value !== undefined),
  );
  if (values.length === 0) return null;
  return Math.min(...values);
}

export function buildSubscriptionUsageSummary(
  record,
  title = '',
  nowSeconds = Date.now() / 1000,
  minimumRequestQuota = getConfiguredPreConsumedQuota(),
) {
  if (!isActiveSubscriptionRecord(record, nowSeconds)) return null;

  const subscription = record?.subscription;
  if (!subscription) return null;
  const effectiveMinimumRequestQuota = Math.max(
    1,
    normalizeNumber(minimumRequestQuota),
  );

  const fiveHourWindowStart = normalizeNumber(
    subscription.five_hour_window_start,
  );
  const weeklyWindowStart = normalizeNumber(subscription.weekly_window_start);
  const nextResetTime = normalizeNumber(subscription.next_reset_time);
  const planUsed = getEffectivePlanUsed(
    subscription.amount_used,
    nextResetTime,
    nowSeconds,
  );
  const fiveHourWindow = getEffectiveRollingWindow(
    subscription.five_hour_used,
    fiveHourWindowStart,
    FIVE_HOUR_SECONDS,
    nowSeconds,
  );
  const weeklyWindow = getEffectiveRollingWindow(
    subscription.weekly_used,
    weeklyWindowStart,
    WEEK_SECONDS,
    nowSeconds,
  );

  const rows = [
    buildUsageRow(
      'plan',
      planUsed,
      normalizeNumber(subscription.amount_total),
      effectiveMinimumRequestQuota,
      nextResetTime > nowSeconds ? nextResetTime : undefined,
    ),
    buildUsageRow(
      'fiveHour',
      fiveHourWindow.used,
      normalizeNumber(subscription.five_hour_limit),
      effectiveMinimumRequestQuota,
      fiveHourWindow.resetAt,
    ),
    buildUsageRow(
      'weekly',
      weeklyWindow.used,
      normalizeNumber(subscription.weekly_limit),
      effectiveMinimumRequestQuota,
      weeklyWindow.resetAt,
    ),
  ];

  return {
    id: subscription.id,
    planId: subscription.plan_id,
    title,
    endTime: subscription.end_time,
    rows,
    lowestRemainingPercent: getLowestRemainingPercent([{ rows }]),
  };
}

export function buildSubscriptionUsageSummaries(
  records = [],
  planTitleMap = new Map(),
  nowSeconds = Date.now() / 1000,
  minimumRequestQuota = getConfiguredPreConsumedQuota(),
) {
  return (records || [])
    .map((record) =>
      buildSubscriptionUsageSummary(
        record,
        planTitleMap.get(record?.subscription?.plan_id) || '',
        nowSeconds,
        minimumRequestQuota,
      ),
    )
    .filter(Boolean);
}
