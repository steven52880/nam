import { Outlet } from 'react-router-dom';
import { api } from '@/api/endpoints';
import { useSession } from '@/hooks/useSession';
import { LayoutProvider } from '@/lib/layout/LayoutProvider';
import type { LayoutDoc } from '@/lib/layout/store';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

// 模块级常量：引用稳定，Provider 不会因为重新渲染而重复加载
const loadLayout = () => api.uiLayout().then((r) => r.layout);
const saveLayout = (doc: LayoutDoc) => api.saveUiLayout(doc);

export function AppShell() {
  // AuthGate 只在会话取到之后才渲染 AppShell，所以这里一定有 user_id；
  // 换用户（key 变化）时整个布局仓库重建并重新加载。
  const session = useSession();
  return (
    <LayoutProvider key={session.identity.user_id} load={loadLayout} save={saveLayout}>
      <div className="flex h-full">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <TopBar />
          <main className="min-h-0 flex-1 overflow-auto">
            <Outlet />
          </main>
        </div>
      </div>
    </LayoutProvider>
  );
}
