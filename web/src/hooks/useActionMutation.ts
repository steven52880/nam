import { useMutation, useQueryClient, type QueryClient, type UseMutationResult } from '@tanstack/react-query';
import { toast } from 'sonner';
import { errorText } from '@/api/errors';

interface ActionMutationOptions<TVars, TData> {
  mutationFn: (vars: TVars) => Promise<TData>;
  /** 成功提示；不传则不提示。 */
  success?: string | ((data: TData, vars: TVars) => string);
  /** 成功后的刷新，通常是 invalidate.xxx。 */
  invalidate?: (qc: QueryClient, data: TData, vars: TVars) => Promise<void> | void;
  onMutate?: (vars: TVars) => void;
  onSuccess?: (data: TData, vars: TVars) => void;
  /** 返回 true 表示已自行处理，不再弹默认的错误提示。 */
  onError?: (err: Error, vars: TVars) => boolean | void;
  onSettled?: () => void;
}

/** 提交助手：全站唯一能写 useMutation 的地方。顺序：invalidate → success 提示 → onSuccess。 */
export function useActionMutation<TVars = void, TData = unknown>(
  opts: ActionMutationOptions<TVars, TData>,
): UseMutationResult<TData, Error, TVars> {
  const qc = useQueryClient();
  return useMutation<TData, Error, TVars>({
    mutationFn: opts.mutationFn,
    onMutate: (vars) => {
      opts.onMutate?.(vars);
    },
    onSuccess: (data, vars) => {
      void opts.invalidate?.(qc, data, vars);
      if (opts.success !== undefined) {
        const msg = typeof opts.success === 'function' ? opts.success(data, vars) : opts.success;
        if (msg) toast.success(msg);
      }
      opts.onSuccess?.(data, vars);
    },
    onError: (err, vars) => {
      const handled = opts.onError?.(err, vars);
      if (handled !== true) toast.error(errorText(err));
    },
    onSettled: () => {
      opts.onSettled?.();
    },
  });
}
