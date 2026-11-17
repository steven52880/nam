import { useCallback, useRef } from 'react';

/** 返回一个引用永远不变、但总是调用最新 fn 的函数（让 memo 组件的 props 保持稳定）。 */
export function useEvent<A extends unknown[], R>(fn: (...args: A) => R): (...args: A) => R {
  const ref = useRef(fn);
  ref.current = fn;
  return useCallback((...args: A) => ref.current(...args), []);
}
