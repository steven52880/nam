import type { RouteObject } from 'react-router-dom';
import { AuthGate } from '@/app/AuthGate';
import AuditPage from '@/pages/Audit';
import ChannelsPage from '@/pages/Channels';
import ForbiddenPage, { NotFoundPage } from '@/pages/Forbidden';
import GridDemoPage from '@/pages/GridDemo';
import LoginPage from '@/pages/Login';
import OverviewPage from '@/pages/Overview';
import { PlaceholderPage } from '@/pages/Placeholder';
import SettingsPage from '@/pages/Settings';
import UpstreamsPage from '@/pages/Upstreams';

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
      placeholder('pricing', '模型与价格', '厂商售价、原厂价与偏差检查。'),
      placeholder('stats', '统计', '按渠道、模型、上游查看请求量、成功率、延迟与成本。'),
      placeholder('routing', '路由策略（预留）', '按成本分层的优先级与权重建议，算法待讨论。'),
      placeholder('notifications', '通知', '通知通道、规则与事件记录。'),
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];
