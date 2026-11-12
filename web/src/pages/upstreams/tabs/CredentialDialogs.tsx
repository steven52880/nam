import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { Credential, CredType, Upstream, UpstreamInput } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CRED_OPTIONS, credOptionsFor, defaultCredType } from '../constants';
import { CredentialFields, Field } from '../UpstreamForm';
import { CheckList } from '../wizard/StepTest';
import { credentialValid, usesUserId } from '../wizard/state';

export function CredentialDialog({ upstream, open, onOpenChange }: { upstream: Upstream; open: boolean; onOpenChange: (open: boolean) => void }) {
  const qc = useQueryClient();
  const [credType, setCredType] = useState<CredType>(upstream.cred_type || defaultCredType(upstream.type));
  const [cred, setCred] = useState<Credential>({});
  const [userId, setUserId] = useState(upstream.platform_user_id ? String(upstream.platform_user_id) : '');

  const options = credOptionsFor(upstream.type);
  if (!options.some(([v]) => v === credType)) {
    const current = CRED_OPTIONS.find(([v]) => v === credType);
    if (current) options.push(current);
  }
  const sessionType = credType === 'cookie' || credType === 'sub2api_session';
  const invalid = sessionType && !credentialValid(upstream.type, credType, cred, userId);

  const save = useMutation({
    mutationFn: () => {
      const body: UpstreamInput = { cred_type: credType, credential: credType ? cred : {} };
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
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>更换凭证</DialogTitle>
          <DialogDescription>新凭证会覆盖旧凭证，保存后建议再测试一次连通性。</DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          <Field label="凭证方式">
            <Select value={credType === '' ? 'none' : credType} onValueChange={(v) => { setCredType(v === 'none' ? '' : (v as CredType)); setCred({}); }}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {options.map(([v, l]) => <SelectItem key={v || 'none'} value={v === '' ? 'none' : v}>{l}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
          <CredentialFields credType={credType} value={cred} onChange={setCred} userId={userId} onUserId={setUserId} upstreamType={upstream.type} />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
          <Button disabled={save.isPending || invalid} onClick={() => save.mutate()}>保存</Button>
        </DialogFooter>
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
