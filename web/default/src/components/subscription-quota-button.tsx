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
import { Gauge } from 'lucide-react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from '@/components/ui/hover-card'
import { Progress } from '@/components/ui/progress'
import { useSubscriptionUsageSummary } from '@/features/subscriptions/hooks/use-subscription-usage-summary'
import type {
  SubscriptionUsageRow,
  SubscriptionUsageSummary,
} from '@/features/subscriptions/lib/usage-summary'

function getPercentStatusClass(percent: number | null): string {
  if (percent === null) return 'bg-primary'
  if (percent <= 10) return 'bg-destructive'
  if (percent <= 25) return 'bg-amber-500'
  return 'bg-primary'
}

function formatRemaining(
  row: SubscriptionUsageRow,
  t: TFunction
): string {
  if (row.isUnlimited || row.remainingPercent === null) return t('Unlimited')
  return t('{{percent}}% remaining', { percent: row.remainingPercent })
}

function UsageRow({ row }: { row: SubscriptionUsageRow }) {
  const { t } = useTranslation()
  const now = Date.now() / 1000
  const resetAt = row.resetAt && row.resetAt > now ? row.resetAt : undefined
  const showActualRemaining =
    !row.isUnlimited &&
    !row.isRequestAvailable &&
    row.actualRemaining !== null &&
    row.actualRemaining > 0 &&
    row.minimumRequestQuota > 0

  return (
    <div className='space-y-1.5'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='text-muted-foreground'>{t(row.labelKey)}</span>
        <span className='text-foreground font-medium tabular-nums'>
          {formatRemaining(row, t)}
        </span>
      </div>
      {!row.isUnlimited && <Progress value={row.progressValue} />}
      {showActualRemaining && (
        <div className='text-muted-foreground text-[11px]'>
          {t('Actual remaining: {{remaining}} · Minimum request reserve: {{required}}', {
            remaining: formatQuota(row.actualRemaining || 0),
            required: formatQuota(row.minimumRequestQuota),
          })}
        </div>
      )}
      {resetAt && (
        <div className='text-muted-foreground text-[11px]'>
          {t('Resets at')} {new Date(resetAt * 1000).toLocaleString()}
        </div>
      )}
    </div>
  )
}

function SubscriptionSummaryBlock({
  summary,
}: {
  summary: SubscriptionUsageSummary
}) {
  const { t } = useTranslation()
  const title = summary.title
    ? `${summary.title} - ${t('Subscription')} #${summary.id}`
    : `${t('Subscription')} #${summary.id}`

  return (
    <div className='space-y-2 rounded-md border p-2.5'>
      <div className='truncate text-xs font-medium'>{title}</div>
      <div className='space-y-2'>
        {summary.rows.map((row) => (
          <UsageRow key={row.key} row={row} />
        ))}
      </div>
    </div>
  )
}

export function SubscriptionQuotaButton() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { summaries, lowestRemainingPercent, isLoading } =
    useSubscriptionUsageSummary({
      includePlanTitles: true,
      enabled: !!user,
    })

  if (!user || isLoading || summaries.length === 0) return null

  const ariaLabel =
    lowestRemainingPercent === null
      ? t('Subscription quota')
      : `${t('Subscription quota')}: ${t('{{percent}}% remaining', {
          percent: lowestRemainingPercent,
        })}`

  return (
    <HoverCard>
      <HoverCardTrigger
        delay={100}
        closeDelay={100}
        render={
          <Button
            variant='ghost'
            size='icon'
            className='relative h-9 w-9'
            aria-label={ariaLabel}
          >
            <Gauge className='size-[1.2rem]' />
            {lowestRemainingPercent !== null && (
              <span
                className={cn(
                  'ring-background absolute top-1 right-1 size-2 rounded-full ring-2',
                  getPercentStatusClass(lowestRemainingPercent)
                )}
                aria-hidden='true'
              />
            )}
          </Button>
        }
      />
      <HoverCardContent
        align='end'
        className='w-80 max-w-[calc(100vw-1rem)] space-y-3 p-3'
      >
        <div className='flex items-center justify-between gap-3'>
          <div className='text-sm font-medium'>{t('Subscription quota')}</div>
          {lowestRemainingPercent !== null && (
            <div className='text-muted-foreground text-xs tabular-nums'>
              {t('{{percent}}% remaining', {
                percent: lowestRemainingPercent,
              })}
            </div>
          )}
        </div>
        <div className='max-h-96 space-y-2 overflow-y-auto pr-1'>
          {summaries.map((summary) => (
            <SubscriptionSummaryBlock key={summary.id} summary={summary} />
          ))}
        </div>
      </HoverCardContent>
    </HoverCard>
  )
}
