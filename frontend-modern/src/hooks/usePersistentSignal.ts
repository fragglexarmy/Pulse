import { Accessor, Setter, createEffect, createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

export type PersistentSignalOptions<T> = {
  /**
   * Custom serialization function. Defaults to `String(value)`.
   */
  serialize?: (value: T) => string;
  /**
   * Custom deserialization function. Defaults to casting the stored string.
   */
  deserialize?: (value: string) => T;
  /**
   * Optional equality comparison passed to Solid's `createSignal`.
   */
  equals?: (prev: T, next: T) => boolean;
  /**
   * Alternate storage implementation (defaults to `window.localStorage`).
   */
  storage?: Storage;
  /**
   * Older storage keys to read from and migrate away from when the primary key changes.
   */
  fallbackKeys?: string[] | Accessor<string[]>;
};

/**
 * Creates a Solid signal that persists its value to localStorage (or a custom storage).
 * The signal reads the initial value synchronously from storage when available.
 */
export function usePersistentSignal<T>(
  key: string | Accessor<string>,
  defaultValue: T,
  options: PersistentSignalOptions<T> = {},
): [Accessor<T>, Setter<T>] {
  const storage =
    options.storage ?? (typeof window !== 'undefined' ? window.localStorage : undefined);
  const serialize = options.serialize ?? ((value: T) => String(value));
  const deserialize = options.deserialize ?? ((value: string) => value as unknown as T);
  const resolveKey = () => (typeof key === 'function' ? (key as Accessor<string>)() : key);
  const resolveFallbackKeys = (resolvedKey: string) => {
    const rawFallbackKeys =
      typeof options.fallbackKeys === 'function'
        ? (options.fallbackKeys as Accessor<string[]>)()
        : (options.fallbackKeys ?? []);
    return rawFallbackKeys.filter((fallbackKey) => fallbackKey !== resolvedKey);
  };
  const readStoredValue = (resolvedKey: string, fallbackKeys: string[]) => {
    if (!storage) {
      return { value: defaultValue, found: false };
    }

    try {
      const raw = storage.getItem(resolvedKey);
      if (raw !== null) {
        return { value: deserialize(raw), found: true };
      }

      for (const fallbackKey of fallbackKeys) {
        const fallbackRaw = storage.getItem(fallbackKey);
        if (fallbackRaw !== null) {
          return { value: deserialize(fallbackRaw), found: true };
        }
      }
    } catch (err) {
      logger.warn(`[usePersistentSignal] Failed to read "${resolvedKey}" from storage`, err);
    }

    return { value: defaultValue, found: false };
  };

  const initialKey = resolveKey();
  const initialFallbackKeys = resolveFallbackKeys(initialKey);
  const initialValue = readStoredValue(initialKey, initialFallbackKeys).value;

  const signalOptions = options.equals ? { equals: options.equals } : undefined;
  const [value, setValue] = createSignal<T>(initialValue, signalOptions);
  let activeKey = initialKey;
  let persistedKey = initialKey;

  createEffect(() => {
    const currentKey = resolveKey();
    const fallbackKeys = resolveFallbackKeys(currentKey);
    if (currentKey === activeKey) {
      return;
    }

    const stored = readStoredValue(currentKey, fallbackKeys);
    if (stored.found) {
      setValue(() => stored.value);
    }
    activeKey = currentKey;
  });

  createEffect(() => {
    const currentKey = resolveKey();
    const fallbackKeys = resolveFallbackKeys(currentKey);
    if (!storage) {
      persistedKey = currentKey;
      return;
    }

    const current = value();
    try {
      if (persistedKey !== currentKey) {
        storage.removeItem(persistedKey);
      }
      if (current === undefined || current === null) {
        storage.removeItem(currentKey);
      } else {
        storage.setItem(currentKey, serialize(current));
      }
      for (const fallbackKey of fallbackKeys) {
        storage.removeItem(fallbackKey);
      }
    } catch (err) {
      logger.warn(`[usePersistentSignal] Failed to persist "${currentKey}"`, err);
    }
    persistedKey = currentKey;
  });

  return [value, setValue];
}
