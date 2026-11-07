import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/endpoints';
import { ApiError } from '@/api/client';
import type { LoginFailure } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

function asFailure(data: unknown): LoginFailure {
  if (typeof data !== 'object' || data === null) return {};
  const d = data as Record<string, unknown>;
  return {
    failures: typeof d.failures === 'number' ? d.failures : undefined,
    locked_until: typeof d.locked_until === 'number' ? d.locked_until : undefined,
  };
}

export default function LoginPage() {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [failures, setFailures] = useState(0);
  const [lockedUntil, setLockedUntil] = useState(0);
  const [now, setNow] = useState(() => Date.now());
  const navigate = useNavigate();
  const qc = useQueryClient();

  useEffect(() => {
    if (!lockedUntil) return;
    const t = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(t);
  }, [lockedUntil]);

  const remaining = Math.max(0, Math.ceil(lockedUntil - now / 1000));
  const locked = remaining > 0;

  const login = useMutation({
    mutationFn: () => api.login(password),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ['session'] });
      navigate('/', { replace: true });
    },
    onError: (e: Error) => {
      if (e instanceof ApiError) {
        // 404：嵌入模式没有 nam 自己的登录接口，转去 NewAPI 登录页
        if (e.status === 404) {
          window.location.href = '/login';
          return;
        }
        const f = asFailure(e.data);
        if (f.failures !== undefined) setFailures(f.failures);
        if (f.locked_until) {
          setLockedUntil(f.locked_until);
          setNow(Date.now());
        }
        setError(e.status === 429 ? '登录已被锁定' : e.status === 401 ? '密码错误' : e.message);
      } else {
        setError(e.message);
      }
      setPassword('');
    },
  });

  return (
    <div className="flex h-full items-center justify-center bg-muted">
      <form
        className="grid w-80 gap-3 rounded-lg border bg-card p-6 shadow-sm"
        onSubmit={(e) => {
          e.preventDefault();
          if (!locked && password) login.mutate();
        }}
      >
        <div className="flex items-center gap-2">
          <span className="flex h-6 w-6 items-center justify-center rounded bg-primary text-sm font-bold text-primary-foreground">n</span>
          <span className="text-xl font-semibold">nam 登录</span>
        </div>
        <p className="text-sm text-muted-foreground">请输入管理员密码。</p>
        <Input
          type="password"
          autoFocus
          placeholder="管理员密码"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={locked}
        />
        {error && (
          <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">
            {error}
            {failures > 0 && !locked && `，已连续失败 ${failures} 次（满 5 次将锁定 60 秒）`}
            {locked && `，请 ${remaining} 秒后重试`}
          </div>
        )}
        <Button type="submit" disabled={locked || !password || login.isPending}>
          {locked ? `已锁定（${remaining} 秒）` : '登录'}
        </Button>
      </form>
    </div>
  );
}
