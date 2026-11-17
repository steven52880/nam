import { Circle } from 'lucide-react';
import { useState } from 'react';
import { lastCall } from '@/api/client';
import { useBackend } from '@/api/queries';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useSession } from '@/hooks/useSession';
import { formatTime } from '@/lib/format';
import { cn } from '@/lib/utils';

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-3 py-1">
      <div className="w-24 shrink-0 text-muted-foreground">{label}</div>
      <div className="min-w-0 flex-1 break-all">{children}</div>
    </div>
  );
}

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
          <div className="text-base">
            <Row label="运行模式">{modeText}</Row>
            <Row label="版本">{backend.data?.version || session.version || '—'}</Row>
            <Row label="访问身份">
              {session.identity.username}（用户 ID {session.identity.user_id}，角色 {session.identity.role}）
            </Row>
            {session.mode === 'standalone' && <Row label="地址">{backend.data?.address || '—'}</Row>}
            <Row label="最近请求">{formatTime(backend.data?.last_request_at ?? 0)}</Row>
            <Row label="最近错误">
              <span className={bad ? 'text-destructive' : ''}>
                {backend.data?.last_error || (backend.isError ? backend.error.message : '') || lastCall.error || '无'}
              </span>
            </Row>
          </div>
          <div className="flex justify-end">
            <Button variant="outline" onClick={() => void backend.refetch()}>
              重新检测
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
