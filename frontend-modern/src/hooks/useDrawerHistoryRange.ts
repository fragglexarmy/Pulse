import type { Accessor } from 'solid-js';
import type { HistoryTimeRange } from '@/api/charts';
import { usePersistentSignal } from './usePersistentSignal';

const DRAWER_HISTORY_RANGE_PREFIX = 'pulse.drawerHistoryRange';

const allowedHistoryRanges = new Set<HistoryTimeRange>([
  '1h',
  '6h',
  '12h',
  '24h',
  '7d',
  '30d',
  '90d',
]);

function normaliseHistoryRange(value: string): HistoryTimeRange {
  return allowedHistoryRanges.has(value as HistoryTimeRange) ? (value as HistoryTimeRange) : '1h';
}

interface DrawerHistoryRangeOptions {
  fallbackKeys?: string[] | Accessor<string[]>;
}

export function useDrawerHistoryRange(
  resourceKey: string | Accessor<string>,
  options: DrawerHistoryRangeOptions = {},
) {
  return usePersistentSignal<HistoryTimeRange>(
    () => `${DRAWER_HISTORY_RANGE_PREFIX}.${typeof resourceKey === 'function' ? resourceKey() : resourceKey}`,
    '1h',
    {
      deserialize: normaliseHistoryRange,
      serialize: (value) => value,
      fallbackKeys: () => {
        const fallbackKeys =
          typeof options.fallbackKeys === 'function'
            ? options.fallbackKeys()
            : (options.fallbackKeys ?? []);
        return fallbackKeys.map(
          (fallbackKey) => `${DRAWER_HISTORY_RANGE_PREFIX}.${fallbackKey}`,
        );
      },
    },
  );
}
