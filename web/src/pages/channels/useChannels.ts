import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText, issueText } from '@/api/errors';
import type { ChannelChange } from '@/api/types';

export const CHANNELS_KEY = ['channels'] as const;

export function useChannelList() {
  return useQuery({ queryKey: CHANNELS_KEY, queryFn: api.channels });
}

/** 提交草稿修改：成功后刷新列表并提示，失败时弹出校验问题（表格随刷新恢复原值）。 */
export function useDraftPatch(successMessage?: (count: number) => string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (changes: ChannelChange[]) => api.patchDrafts(changes),
    onSuccess: (data, changes) => {
      if (successMessage) toast.success(successMessage(changes.length));
      if (data.issues && data.issues.length > 0) toast.warning(data.issues.map(issueText).join('；'));
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: (e: Error) => {
      toast.error(errorText(e));
      void qc.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
  });
}
