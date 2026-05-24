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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, Popover, Progress, Typography } from '@douyinfe/semi-ui';
import { Gauge } from 'lucide-react';
import { API, renderQuota } from '../../../helpers';
import {
  buildSubscriptionUsageSummaries,
  getLowestRemainingPercent,
} from '../../../helpers/subscriptionUsage';

const { Text } = Typography;

function getPercentStatusClass(percent) {
  if (percent === null) return 'bg-purple-500';
  if (percent <= 10) return 'bg-red-500';
  if (percent <= 25) return 'bg-amber-500';
  return 'bg-purple-500';
}

function formatRemaining(row, t) {
  if (row.isUnlimited || row.remainingPercent === null) return t('不限');
  return t('剩余 {{percent}}%', { percent: row.remainingPercent });
}

function UsageRow({ row, t }) {
  const resetAt =
    row.resetAt && row.resetAt > Date.now() / 1000 ? row.resetAt : undefined;
  const showActualRemaining =
    !row.isUnlimited &&
    !row.isRequestAvailable &&
    row.actualRemaining !== null &&
    row.actualRemaining > 0 &&
    row.minimumRequestQuota > 0;

  return (
    <div className='space-y-1'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <Text type='tertiary'>{t(row.labelKey)}</Text>
        <Text strong className='tabular-nums'>
          {formatRemaining(row, t)}
        </Text>
      </div>
      {!row.isUnlimited && (
        <Progress percent={row.progressValue} showInfo={false} size='small' />
      )}
      {showActualRemaining && (
        <div className='text-[11px] text-semi-color-text-2'>
          {t('实际剩余：{{remaining}} · 最低预扣：{{required}}', {
            remaining: renderQuota(row.actualRemaining || 0),
            required: renderQuota(row.minimumRequestQuota),
          })}
        </div>
      )}
      {resetAt && (
        <div className='text-[11px] text-semi-color-text-2'>
          {t('下一次重置')}: {new Date(resetAt * 1000).toLocaleString()}
        </div>
      )}
    </div>
  );
}

function SummaryBlock({ summary, t }) {
  const title = summary.title
    ? `${summary.title} · ${t('订阅')} #${summary.id}`
    : `${t('订阅')} #${summary.id}`;

  return (
    <div className='space-y-2 rounded-md border border-semi-color-border p-2.5'>
      <Text strong size='small' ellipsis={{ showTooltip: true }}>
        {title}
      </Text>
      <div className='space-y-2'>
        {summary.rows.map((row) => (
          <UsageRow key={row.key} row={row} t={t} />
        ))}
      </div>
    </div>
  );
}

const SubscriptionQuotaButton = ({ userState, t }) => {
  const [loading, setLoading] = useState(false);
  const [activeSubscriptions, setActiveSubscriptions] = useState([]);
  const [plans, setPlans] = useState([]);
  const user = userState?.user;

  useEffect(() => {
    let cancelled = false;

    async function loadSubscriptionUsage() {
      if (!user?.id) {
        setActiveSubscriptions([]);
        setPlans([]);
        return;
      }

      setLoading(true);
      try {
        const [selfRes, plansRes] = await Promise.all([
          API.get('/api/subscription/self', { skipErrorHandler: true }),
          API.get('/api/subscription/plans', { skipErrorHandler: true }),
        ]);
        if (cancelled) return;

        setActiveSubscriptions(
          selfRes.data?.success ? selfRes.data.data?.subscriptions || [] : [],
        );
        setPlans(plansRes.data?.success ? plansRes.data.data || [] : []);
      } catch (e) {
        if (!cancelled) {
          setActiveSubscriptions([]);
          setPlans([]);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadSubscriptionUsage();

    return () => {
      cancelled = true;
    };
  }, [user?.id]);

  const planTitleMap = useMemo(() => {
    const map = new Map();
    (plans || []).forEach((item) => {
      if (item?.plan?.id) {
        map.set(item.plan.id, item.plan.title || '');
      }
    });
    return map;
  }, [plans]);

  const summaries = useMemo(
    () => buildSubscriptionUsageSummaries(activeSubscriptions, planTitleMap),
    [activeSubscriptions, planTitleMap],
  );

  const lowestRemainingPercent = useMemo(
    () => getLowestRemainingPercent(summaries),
    [summaries],
  );

  if (!user?.id || loading || summaries.length === 0) return null;

  const content = (
    <div className='w-80 max-w-[calc(100vw-1rem)] space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <Text strong>{t('订阅额度')}</Text>
        {lowestRemainingPercent !== null && (
          <Text type='tertiary' size='small' className='tabular-nums'>
            {t('剩余 {{percent}}%', { percent: lowestRemainingPercent })}
          </Text>
        )}
      </div>
      <div className='max-h-96 space-y-2 overflow-y-auto pr-1'>
        {summaries.map((summary) => (
          <SummaryBlock key={summary.id} summary={summary} t={t} />
        ))}
      </div>
    </div>
  );

  return (
    <Popover content={content} position='bottomRight' trigger='hover' showArrow>
      <span className='relative inline-flex'>
        <Button
          icon={<Gauge size={18} />}
          aria-label={t('订阅额度')}
          theme='borderless'
          type='tertiary'
          className='!p-1.5 !text-current focus:!bg-semi-color-fill-1 dark:focus:!bg-gray-700 !rounded-full !bg-semi-color-fill-0 dark:!bg-semi-color-fill-1 hover:!bg-semi-color-fill-1 dark:hover:!bg-semi-color-fill-2'
        />
        {lowestRemainingPercent !== null && (
          <span
            className={`absolute right-0.5 top-0.5 size-2 rounded-full ring-2 ring-white dark:ring-zinc-900 ${getPercentStatusClass(
              lowestRemainingPercent,
            )}`}
            aria-hidden='true'
          />
        )}
      </span>
    </Popover>
  );
};

export default SubscriptionQuotaButton;
