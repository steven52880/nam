import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { Credential, CredType, Upstream, UpstreamInput, UpstreamType } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { allowsCredType, defaultCredType } from '../constants';
import { CredentialPicker } from '../UpstreamForm';
import { CheckList } from '../wizard/StepTest';
import { credentialValid, usesUserId } from '../wizard/state';

function initialCredType(upstream: Upstream, type: UpstreamType): CredType {
  if (type === upstream.type) return upstream.cred_type || defaultCredType(type);
  return allowsCredType(type, upstream.cred_type) ? upstream.cred_type : defaultCredType(type);
}

/** 表单放在 DialogContent 内：对话框每次打开才挂载，状态随当前 upstream 与有效类型重新初始化。 */
function CredentialForm({ upstream, type, onOpenChange }: { upstream: Upstream; type: UpstreamType; onOpenChange: (open: boolean) => void }) {
  const qc = useQueryClient();
  const [credType, setCredType] = useState<CredType>(() => initialCredType(upstream, type));
  const [cred, setCred] = useState<Credential>({});
  const [userId, setUserId] = useState(upstream.platform_user_id ? String(upstream.platform_user_id) : '');

  const typeChanged = type !== upstream.type;
  const sessionType = credType === 'cookie' || credType === 'sub2api_session';
  const invalid = sessionType && !credentialValid(type, credType, cred, userId);

  const save = useMutation({
    mutationFn: () => {
      const body: UpstreamInput = { cred_type: credType, credential: credType ? cred : {} };
      if (typeChanged) body.type = type;
      if (usesUserId(credType) && userId) body.platform_user_id = Number(userId);
      return api.updateUpstream(upstream.id, body);
    },
    onSuccess: () => {
      toast.success('凭证已更新');
      void qc.invalidateQueries({ queryKey: ['upstreams'] });
      onOpenChange(false);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>更换凭证</DialogTitle>
        <DialogDescription>新凭证会覆盖旧凭证，保存后建议再测试一次连通性。</DialogDescription>
      </DialogHeader>
      <div className="grid gap-3">
        <CredentialPicker
          upstreamType={type}
          credType={credType}
          onCredType={(t) => { setCredType(t); setCred({}); }}
          value={cred}
          onChange={setCred}
          userId={userId}
          onUserId={setUserId}
        />
        {typeChanged && <p className="text-xs text-muted-foreground">类型将一并保存为 {type}</p>}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
        <Button disabled={save.isPending || invalid} onClick={() => save.mutate()}>保存</Button>
      </DialogFooter>
    </>
  );
}

/** type：有效的上游类型（如抽屉里尚未保存的类型），缺省为已保存的 upstream.type。 */
export function CredentialDialog({ upstream, type, open, onOpenChange }: { upstream: Upstream; type?: UpstreamType; open: boolean; onOpenChange: (open: boolean) => void }) {
  const effectiveType = type ?? upstream.type;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <CredentialForm key={`${upstream.id}-${effectiveType}`} upstream={upstream} type={effectiveType} onOpenChange={onOpenChange} />
      </DialogContent>
    </Dialog>
  );
}

export function TestUpstreamDialog({ upstream, open, onOpenChange }: { upstream: Upstream; open: boolean; onOpenChange: (open: boolean) => void }) {
  const test = useMutation({ mutationFn: () => api.testUpstream(upstream.id) });
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (next) test.mutate();
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>测试连通性</DialogTitle>
          <DialogDescription>{upstream.name}</DialogDescription>
        </DialogHeader>
        <CheckList results={test.data ?? []} pending={test.isPending} error={test.isError ? errorText(test.error) : undefined} />
        <DialogFooter>
          <Button variant="outline" disabled={test.isPending} onClick={() => test.mutate()}>重新测试</Button>
          <Button onClick={() => onOpenChange(false)}>关闭</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
