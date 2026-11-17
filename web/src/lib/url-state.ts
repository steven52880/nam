import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

/** 纯函数：在 prev 的基础上应用修改，null 表示删除，其余参数保持不变。 */
export function applyUrlPatch(prev: URLSearchParams, patch: Record<string, string | null>): URLSearchParams {
  const next = new URLSearchParams(prev);
  for (const [key, value] of Object.entries(patch)) {
    if (value === null) next.delete(key);
    else next.set(key, value);
  }
  return next;
}

/**
 * 网址参数。`patch` 一次只调用一次 setSearchParams，保证多个参数同时生效
 * （连续调用两次 setSearchParams 时，第二次会基于旧地址覆盖第一次）。
 */
export function useUrlState(): {
  get: (key: string) => string | null;
  patch: (patch: Record<string, string | null>, opts?: { replace?: boolean }) => void;
} {
  const [params, setSearchParams] = useSearchParams();
  const get = useCallback((key: string) => params.get(key), [params]);
  const patch = useCallback(
    (change: Record<string, string | null>, opts?: { replace?: boolean }) => {
      setSearchParams((prev) => applyUrlPatch(prev, change), { replace: opts?.replace ?? true });
    },
    [setSearchParams],
  );
  return { get, patch };
}
