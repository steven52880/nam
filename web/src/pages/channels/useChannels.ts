import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { issueText } from '@/api/errors';
import { invalidate } from '@/api/invalidate';
import type { ChannelChange } from '@/api/types';
import { useActionMutation } from '@/hooks/useActionMutation';

/** 提交草稿修改：成功后刷新列表并提示，失败时弹出校验问题（表格随刷新恢复原值）。 */
export function useDraftPatch(successMessage?: (count: number) => string) {
  const qc = useQueryClient();
  return useActionMutation({
    mutationFn: (changes: ChannelChange[]) => api.patchDrafts(changes),
    success: successMessage ? (_data, changes) => successMessage(changes.length) : undefined,
    invalidate: invalidate.channels,
    onSuccess: (data) => {
      if (data.issues && data.issues.length > 0) toast.warning(data.issues.map(issueText).join('；'));
    },
    onError: () => {
      void invalidate.channels(qc);
      return false;
    },
  });
}
