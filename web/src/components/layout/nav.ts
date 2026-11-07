import {
  BarChart3,
  Bell,
  Boxes,
  GitBranch,
  History,
  LayoutDashboard,
  Network,
  Settings,
  Tags,
  type LucideIcon,
} from 'lucide-react';

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  badge?: 'upstreams';
  reserved?: boolean;
  description: string;
}

export interface NavGroup {
  title: string;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    title: '监控',
    items: [
      { to: '/', label: '总览', icon: LayoutDashboard, description: '告警、指标、后台任务与最近写入' },
      { to: '/stats', label: '统计', icon: BarChart3, description: '按渠道、模型、上游查看请求量、成功率、延迟与成本' },
    ],
  },
  {
    title: '配置',
    items: [
      { to: '/upstreams', label: '上游', icon: Network, badge: 'upstreams', description: '管理上游站点、凭证与上游货币汇率' },
      { to: '/channels', label: '渠道', icon: Boxes, description: '渠道表格、草稿与应用变更' },
      { to: '/pricing', label: '模型与价格', icon: Tags, description: '厂商售价、原厂价与偏差检查' },
      { to: '/routing', label: '路由策略', icon: GitBranch, reserved: true, description: '按成本分层的优先级与权重建议（预留）' },
    ],
  },
  {
    title: '运维',
    items: [
      { to: '/audit', label: '写入历史与审计', icon: History, description: '所有写入的差异、请求与回滚' },
      { to: '/notifications', label: '通知', icon: Bell, description: '通知通道、规则与事件记录' },
      { to: '/settings', label: '设置', icon: Settings, description: '连接、定价参数、调度间隔与日志保留' },
    ],
  },
];

export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);
