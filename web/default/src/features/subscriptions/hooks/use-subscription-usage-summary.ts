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
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getPublicPlans, getSelfSubscriptionFull } from '../api'
import {
  buildSubscriptionUsageSummaries,
  getLowestRemainingPercent,
  publicSubscriptionPlansQueryKey,
  selfSubscriptionQueryKey,
} from '../lib/usage-summary'

interface UseSubscriptionUsageSummaryOptions {
  includePlanTitles?: boolean
  enabled?: boolean
}

export function useSubscriptionUsageSummary(
  options: UseSubscriptionUsageSummaryOptions = {}
) {
  const includePlanTitles = options.includePlanTitles ?? false
  const enabled = options.enabled ?? true
  const { preConsumedQuota } = useSystemConfig()

  const selfQuery = useQuery({
    queryKey: selfSubscriptionQueryKey,
    queryFn: getSelfSubscriptionFull,
    enabled,
    staleTime: 30 * 1000,
  })

  const activeRecords = useMemo(
    () =>
      enabled && selfQuery.data?.success
        ? selfQuery.data.data?.subscriptions || []
        : [],
    [enabled, selfQuery.data]
  )

  const plansQuery = useQuery({
    queryKey: publicSubscriptionPlansQueryKey,
    queryFn: getPublicPlans,
    enabled: enabled && includePlanTitles && activeRecords.length > 0,
    staleTime: 60 * 1000,
  })

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    if (!includePlanTitles || !plansQuery.data?.success) return map

    for (const item of plansQuery.data.data || []) {
      if (item?.plan?.id) {
        map.set(item.plan.id, item.plan.title || '')
      }
    }

    return map
  }, [includePlanTitles, plansQuery.data])

  const summaries = useMemo(
    () =>
      buildSubscriptionUsageSummaries(
        activeRecords,
        planTitleMap,
        Date.now() / 1000,
        preConsumedQuota
      ),
    [activeRecords, planTitleMap, preConsumedQuota]
  )

  return {
    summaries,
    lowestRemainingPercent: getLowestRemainingPercent(summaries),
    isLoading:
      selfQuery.isLoading ||
      (includePlanTitles && activeRecords.length > 0 && plansQuery.isLoading),
    refetch: selfQuery.refetch,
  }
}
