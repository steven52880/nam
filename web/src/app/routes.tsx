import { Suspense, lazy } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import { AuthGate } from '@/app/AuthGate';
import AuditPage from '@/pages/Audit';
import ChannelsPage from '@/pages/Channels';
import ForbiddenPage, { NotFoundPage } from '@/pages/Forbidden';
import GridDemoPage from '@/pages/GridDemo';
import LoginPage from '@/pages/Login';
import OverviewPage from '@/pages/Overview';
import { PlaceholderPage } from '@/pages/Placeholder';
import PricingPage from '@/pages/Pricing';
import SettingsPage from '@/pages/Settings';
import UpstreamsPage from '@/pages/Upstreams';

const StatsPage = lazy(() => import('@/pages/Stats'));

const placeholder = (path: string, title: string, description: string): RouteObject => ({
  path,
  element: <PlaceholderPage title={title} description={description} />,
});

export const routes: RouteObject[] = [
  { path: '/login', element: <LoginPage /> },
  { path: '/forbidden', element: <ForbiddenPage /> },
  {
    element: <AuthGate />,
    children: [
      { index: true, element: <OverviewPage /> },
      { path: 'upstreams', element: <UpstreamsPage /> },
      { path: 'audit', element: <AuditPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'dev/grid', element: <GridDemoPage /> },
      { path: 'channels', element: <ChannelsPage /> },
      { path: 'pricing', element: <PricingPage /> },
      { path: 'models', element: <Navigate to="/pricing" replace /> },
      {
        path: 'stats',
        element: (
          <Suspense fallback={<div className="p-4 text-muted-foreground">加载统计页…</div>}>
            <StatsPage />
          </Suspense>
        ),
      },
      placeholder('routing', '路由策略（预留）', '按成本分层的优先级与权重建议，算法待讨论。'),
      placeholder('notifications', '通知', '通知通道、规则与事件记录。'),
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];
