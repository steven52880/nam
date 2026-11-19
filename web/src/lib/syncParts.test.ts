import { describe, expect, it } from 'vitest';
import type { SyncPart } from '@/api/types';
import { syncPartView, syncPartViews } from './syncParts';

const NOW = 1_800_000_000_000;
const part = (over: Partial<SyncPart>): SyncPart => ({ state: 'ok', at: NOW / 1000 - 600, message: '', count: null, ...over });

describe('syncPartView', () => {
  it('成功：绿勾，带数量和相对时间', () => {
    expect(syncPartView('groups', part({ count: 13 }), NOW)).toEqual({
      key: 'groups',
      label: '分组倍率',
      status: 'ok',
      reason: '',
      countText: '13 个分组',
      agoText: '10 分钟前',
    });
  });
  it('余额没有数量', () => {
    expect(syncPartView('balance', part({}), NOW).countText).toBe('');
  });
  it('拉取失败：红叉，写原因', () => {
    const v = syncPartView('tokens', part({ state: 'fail', message: 'HTTP 401' }), NOW);
    expect(v.status).toBe('bad');
    expect(v.reason).toBe('HTTP 401');
  });
  it('没有原因时用默认说明', () => {
    expect(syncPartView('tokens', part({ state: 'fail' }), NOW).reason).toBe('拉取失败');
    expect(syncPartView('tokens', part({ state: 'unsupported' }), NOW).reason).toBe('该上游类型不支持');
    expect(syncPartView('model_prices', part({ state: 'empty', count: 0 }), NOW).reason).toBe('没有数据');
  });
  it('没有数据：红叉，带数量 0', () => {
    const v = syncPartView('model_prices', part({ state: 'empty', count: 0, message: '上游没有提供逐模型价格' }), NOW);
    expect(v).toMatchObject({ status: 'bad', reason: '上游没有提供逐模型价格', countText: '0 条价格' });
  });
  it('还没同步：灰色', () => {
    expect(syncPartView('balance', undefined, NOW)).toEqual({
      key: 'balance',
      label: '余额',
      status: 'none',
      reason: '还没同步',
      countText: '',
      agoText: '',
    });
  });
});

describe('syncPartViews', () => {
  it('固定四项顺序', () => {
    expect(syncPartViews({ groups: part({ count: 2 }) }, NOW).map((v) => v.key)).toEqual([
      'balance',
      'tokens',
      'groups',
      'model_prices',
    ]);
  });
  it('没有 sync_parts 时四项都是还没同步', () => {
    expect(syncPartViews(null, NOW).every((v) => v.status === 'none')).toBe(true);
    expect(syncPartViews(undefined, NOW)).toHaveLength(4);
  });
});
