import type { CredType, FailClass, JobName, PriceStatus, StatsWindow, SyncState, UpstreamType, VendorPricingMode, VendorPricingState } from '@/api/types';
import { parseCustomWindow } from './statsWindow';

/** 徽标颜色，与 components/ui/misc.tsx 的 Badge tone 一一对应。 */
export type Tone = 'success' | 'warning' | 'danger' | 'neutral' | 'accent';

export interface LabelDef {
  label: string;
  tone: Tone;
  hint?: string;
}

const def = (label: string, tone: Tone = 'neutral', hint?: string): LabelDef => (hint ? { label, tone, hint } : { label, tone });

/** 渠道状态（NewAPI 的 status 数字）。 */
export const CHANNEL_STATUS: Record<number, LabelDef> = {
  1: def('启用', 'success'),
  2: def('手动禁用'),
  3: def('自动禁用', 'danger'),
};

export const SYNC_STATE: Record<SyncState, LabelDef> = {
  synced: def('已同步', 'success'),
  draft: def('草稿', 'accent'),
  draft_create: def('草稿·新建', 'accent'),
  draft_delete: def('草稿·删除', 'danger'),
  drift: def('漂移', 'warning'),
  error: def('错误', 'danger'),
};

/** 令牌状态：1 启用，其余一律「停用」（调用方先归一成 1 / 0）。 */
export const TOKEN_STATUS: Record<number, LabelDef> = {
  1: def('启用', 'success'),
  0: def('停用'),
};

/** 模型价格状态。后端仍会返回 deviation，界面上一律当作正常，不单独显示。 */
export type VisiblePriceStatus = Exclude<PriceStatus, 'deviation'>;

export const PRICE_STATUS: Record<VisiblePriceStatus, LabelDef> = {
  normal: def('正常', 'success'),
  no_price: def('缺价', 'warning'),
  per_request: def('按次计费', 'accent'),
};

/** 把后端的价格状态换成界面显示用的状态：偏差 → 正常。 */
export function visiblePriceStatus(status: PriceStatus): VisiblePriceStatus {
  return status === 'deviation' ? 'normal' : status;
}

/** 厂商定价状态。 */
export const VENDOR_PRICING_STATE: Record<VendorPricingState, LabelDef> = {
  synced: def('已同步', 'success'),
  to_write: def('待写入', 'accent', '目标倍率与 NewAPI 当前倍率不同，点「预览写入」后写入'),
  pending: def('待确认', 'warning', '变化超过变化保护阈值，需要确认后才写入'),
  drift: def('漂移', 'danger', 'NewAPI 里的倍率被人改过，与 nam 上次写入的不一致'),
  no_channel: def('无可用渠道', 'neutral', '该分组下没有启用且有上游渠道售价的渠道'),
  unset: def('未设置', 'neutral', '还没有填写售价'),
};

/** 厂商定价模式。 */
export const PRICING_MODE: Record<VendorPricingMode, LabelDef> = {
  fixed: def('固定'),
  follow: def('跟随'),
};

export type JobStatusKey = 'running' | 'queued' | 'ok' | 'fail' | 'idle';

/** 后台任务的运行结果徽标。 */
export const JOB_STATUS: Record<JobStatusKey, LabelDef> = {
  running: def('运行中', 'accent'),
  queued: def('排队中', 'warning'),
  ok: def('成功', 'success'),
  fail: def('失败', 'danger'),
  idle: def('未运行'),
};

export const AUDIT_KIND: Record<string, LabelDef> = {
  channel: def('渠道'),
  price: def('价格'),
  setting: def('设置'),
  upstream: def('上游'),
  cache_test: def('缓存测试'),
};

export const AUDIT_RESULT: Record<'ok' | 'fail' | 'skipped', LabelDef> = {
  ok: def('成功', 'success'),
  fail: def('失败', 'danger'),
  skipped: def('已跳过'),
};

export const UPSTREAM_TYPE: Record<UpstreamType, LabelDef> = {
  newapi: def('newapi'),
  'sk-key': def('sk-key'),
  sub2api: def('sub2api'),
  onehub: def('onehub'),
  official: def('official'),
  generic: def('generic'),
  rightcode: def('rightcode'),
};

/** 凭证方式的短名称（徽标用）；空串（暂不填写）不在表里。 */
export const CREDENTIAL_TYPE: Record<Exclude<CredType, ''>, LabelDef> = {
  access_token: def('Access Token', 'success'),
  password: def('账号密码', 'success'),
  sk_key: def('sk-key', 'success'),
  cookie: def('会话 Cookie', 'success'),
  sub2api_session: def('登录令牌（自动续期）', 'success'),
};

/** NewAPI 渠道类型：58 透传、24 Gemini 原生。 */
export const CHANNEL_TYPE: Record<58 | 24, LabelDef> = {
  58: def('透传（58）'),
  24: def('Gemini 原生（24）'),
};

/** 统计时长预设（下拉里固定显示的几项）；自定义时长用 statsWindowLabel。 */
export const STATS_WINDOW_PRESETS: Record<'latest' | '3d' | '7d', LabelDef> = {
  latest: def('最新'),
  '3d': def('3 天'),
  '7d': def('7 天'),
};

/** 统计时长的文字：latest → 最新，12h → 12 小时，30d → 30 天；认不出的原样返回。 */
export function statsWindowLabel(window: StatsWindow): string {
  const preset = (STATS_WINDOW_PRESETS as Record<string, LabelDef | undefined>)[window];
  if (preset) return preset.label;
  const custom = parseCustomWindow(window);
  if (!custom) return window;
  return `${custom.amount} ${custom.unit === 'h' ? '小时' : '天'}`;
}

export const FAIL_CLASS: Record<FailClass, LabelDef> = {
  '429': def('429 限流', 'danger'),
  '5xx': def('5xx', 'danger'),
  timeout: def('超时', 'danger'),
  '401': def('401 / 403', 'danger'),
  balance: def('余额不足', 'danger'),
  other: def('其他', 'danger'),
};

/** 失败类别的展示顺序（对象的整数键会被 JS 排到最前，所以单独写一份）。 */
export const FAIL_CLASS_ORDER: FailClass[] = ['429', '5xx', 'timeout', '401', 'balance', 'other'];

export const JOB_TITLES: Record<JobName, string> = {
  balance: '余额同步',
  ratio: '倍率同步',
  price_book: '价格库同步',
  announcements: '公告检查',
  pull_logs: '日志拉取',
  pricing: '定价计算',
  consistency: '一致性检查',
  cleanup: '数据清理',
  backup: '备份',
};

/** 各上游类型的能力：remoteToken = 适配器支持远程创建令牌。 */
export const UPSTREAM_CAPS: Record<UpstreamType, { remoteToken: boolean }> = {
  newapi: { remoteToken: true },
  'sk-key': { remoteToken: false },
  sub2api: { remoteToken: false },
  onehub: { remoteToken: true },
  official: { remoteToken: false },
  generic: { remoteToken: false },
  rightcode: { remoteToken: true },
};

/** 查名称；表里没有时用键本身做文字、neutral 颜色。 */
export function labelOf(dict: Readonly<Record<string | number, LabelDef>>, key: string | number): LabelDef {
  return dict[key] ?? { label: String(key), tone: 'neutral' };
}

/**
 * 下拉选项 [值, 文字]。默认按字典的键顺序；整数键（如 58 / 24）会被排到前面，
 * 需要别的顺序、或只要其中几项时传 keys。
 */
export function optionsOf(
  dict: Readonly<Record<string | number, LabelDef>>,
  keys?: ReadonlyArray<string | number>,
): Array<[string, string]> {
  const list = keys ?? Object.keys(dict);
  return list.map((k) => [String(k), labelOf(dict, k).label]);
}
