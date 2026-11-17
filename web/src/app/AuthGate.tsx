import { Navigate } from 'react-router-dom';
import { ApiError } from '@/api/client';
import { useSessionQuery } from '@/api/queries';
import { SessionContext } from '@/hooks/useSession';
import { AppShell } from '@/components/layout/AppShell';
import ForbiddenPage from '@/pages/Forbidden';

/** 启动时检查 /session：401 → 登录页，403 → 无权限页。 */
export function AuthGate() {
  const q = useSessionQuery();

  if (q.isLoading) {
    return <div className="flex h-full items-center justify-center text-muted-foreground">加载中…</div>;
  }
  if (q.error instanceof ApiError) {
    if (q.error.status === 401) return <Navigate to="/login" replace />;
    if (q.error.status === 403) return <ForbiddenPage />;
  }
  if (q.isError || !q.data) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2">
        <div className="text-lg font-semibold">无法连接 nam 后端</div>
        <div className="text-muted-foreground">{q.error?.message ?? '未知错误'}</div>
        <button className="text-primary hover:underline" onClick={() => void q.refetch()}>
          重试
        </button>
      </div>
    );
  }
  return (
    <SessionContext.Provider value={q.data}>
      <AppShell />
    </SessionContext.Provider>
  );
}
