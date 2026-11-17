import { Circle } from 'lucide-react';
import { useState } from 'react';
import { lastCall } from '@/api/client';
import { useBackend } from '@/api/queries';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { DescRow } from '@/components/ui/section';
import { useSession } from '@/hooks/useSession';
import { formatTime } from '@/lib/format';
import { cn } from '@/lib/utils';

export function ConnectionStatus() {
  const [open, setOpen] = useState(false);
  const session = useSession();
  const backend = useBackend();

  const bad = backend.isError || Boolean(backend.data?.last_error);
  const label = backend.isLoading ? '连接中' : bad ? 'NewAPI 异常' : 'NewAPI 已连接';
  const modeText = session.mode === 'embedded' ? '嵌入模式' : '独立模式';

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex h-7 items-center gap-1.5 rounded border border-input bg-background px-2 text-base hover:bg-accent"
      >
        <Circle
          className={cn('h-2 w-2 fill-current', bad ? 'text-destructive' : backend.isLoading ? 'text-warning' : 'text-success')}
        />
        {label}
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>NewAPI 连接状态</DialogTitle>
            <DialogDescription>运行方式、版本与最近一次请求。</DialogDescription>
          </DialogHeader>
          <DialogBody className="grid gap-2 break-all text-base">
            <DescRow labelClassName="w-24" label="运行模式">{modeText}</DescRow>
            <DescRow labelClassName="w-24" label="版本">{backend.data?.version || session.version || '—'}</DescRow>
            <DescRow labelClassName="w-24" label="访问身份">
              {session.identity.username}（用户 ID {session.identity.user_id}，角色 {session.identity.role}）
            </DescRow>
            {session.mode === 'standalone' && <DescRow labelClassName="w-24" label="地址">{backend.data?.address || '—'}</DescRow>}
            <DescRow labelClassName="w-24" label="最近请求">{formatTime(backend.data?.last_request_at ?? 0)}</DescRow>
            <DescRow labelClassName="w-24" label="最近错误">
              <span className={bad ? 'text-destructive' : ''}>
                {backend.data?.last_error || (backend.isError ? backend.error.message : '') || lastCall.error || '无'}
              </span>
            </DescRow>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => void backend.refetch()}>
              重新检测
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
