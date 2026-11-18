import { useCallback, useEffect, useState } from 'react';

type ReadableStorage = Pick<Storage, 'getItem'>;
type WritableStorage = Pick<Storage, 'setItem'>;

/** 取浏览器的 localStorage；隐私模式等拿不到（或访问会抛错）时返回 null。 */
function browserStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

/** 读一个 JSON 值；没有、解析失败或没通过 isValid 时返回 fallback。 */
export function readStored<T>(
  storage: ReadableStorage | null,
  key: string,
  fallback: T,
  isValid?: (value: unknown) => value is T,
): T {
  if (!storage) return fallback;
  try {
    const raw = storage.getItem(key);
    if (raw === null) return fallback;
    const parsed: unknown = JSON.parse(raw);
    if (isValid && !isValid(parsed)) return fallback;
    return parsed as T;
  } catch {
    return fallback;
  }
}

/** 写一个 JSON 值；失败（配额、禁用）时静默忽略。 */
export function writeStored(storage: WritableStorage | null, key: string, value: unknown): void {
  if (!storage) return;
  try {
    storage.setItem(key, JSON.stringify(value));
  } catch {
    // 只是记住界面偏好，写不进去不影响使用
  }
}

export const isBoolean = (value: unknown): value is boolean => typeof value === 'boolean';

/**
 * 像 useState，但值会存进浏览器的 localStorage（只在本机浏览器里记住，不进布局仓库）。
 * 每个 key 只应该在一个组件里使用；不监听其它标签页的变化。
 */
export function useLocalStorageState<T>(
  key: string,
  fallback: T,
  isValid?: (value: unknown) => value is T,
): [T, (next: T) => void] {
  const [value, setValue] = useState<T>(() => readStored(browserStorage(), key, fallback, isValid));
  useEffect(() => {
    writeStored(browserStorage(), key, value);
  }, [key, value]);
  const set = useCallback((next: T) => setValue(next), []);
  return [value, set];
}
