import { describe, expect, it } from 'vitest';
import type { VendorGroupsPlan, VendorPricingItem, VendorPricingState } from '@/api/types';
import {
  attentionCount,
  attentionSummary,
  canPreviewWrite,
  cnyPerUsd,
  currentPrice,
  groupRatioToText,
  hasDraft,
  needsAttention,
  parseGroupRatioInput,
  parseMarkupInput,
  parseMinChange,
  parsePercentSetting,
  parseSellPriceInput,
  parseThreshold,
  percentTextToRatio,
  prettyJson,
  ratioToPercentText,
  rawRatioTitle,
  signedChange,
  targetGroupRatio,
} from './vendorPricing';

const item = (state: VendorPricingState, extra: Partial<VendorPricingItem> = {}): VendorPricingItem => ({
  vendor_id: 1,
  vendor: 'A',
  group: 'a',
  order: 0,
  mode: 'follow',
  markup: 1.5,
  sell_price: null,
  reference: null,
  target_price: null,
  target_ratio: null,
  newapi_ratio: null,
  draft_ratio: null,
  change: null,
  small_change: false,
  state,
  revenue_7d: null,
  cost_7d: null,
  margin_7d: null,
  ...extra,
});

const groups = (changed: boolean): VendorGroupsPlan => ({
  auto_groups: { current: [], target: [] },
  user_usable_groups: { current: {}, target: {} },
  changed,
});

describe('percentTextToRatio', () => {
  it('百分比文字转小数', () => {
    expect(percentTextToRatio('30')).toBe(0.3);
    expect(percentTextToRatio(' 30% ')).toBe(0.3);
    expect(percentTextToRatio('2.15')).toBe(0.0215);
    expect(percentTextToRatio('.5')).toBe(0.005);
  });
  it('保留到 0.01%', () => {
    expect(percentTextToRatio('30.0049')).toBe(0.3);
    expect(percentTextToRatio('30.006')).toBe(0.3001);
  });
  it('不合法返回 null', () => {
    expect(percentTextToRatio('')).toBeNull();
    expect(percentTextToRatio('abc')).toBeNull();
    expect(percentTextToRatio('-5')).toBeNull();
    expect(percentTextToRatio('1e3')).toBeNull();
  });
});

describe('ratioToPercentText', () => {
  it('小数转输入框文字', () => {
    expect(ratioToPercentText(0.3)).toBe('30');
    expect(ratioToPercentText(0.0215)).toBe('2.15');
    expect(ratioToPercentText(0.5)).toBe('50');
  });
});

describe('parseSellPriceInput', () => {
  it('合法', () => {
    expect(parseSellPriceInput('30')).toEqual({ ok: true, value: 0.3 });
    expect(parseSellPriceInput('1000')).toEqual({ ok: true, value: 10 });
  });
  it('0、负数、超上限、非数字都报错', () => {
    expect(parseSellPriceInput('0').ok).toBe(false);
    expect(parseSellPriceInput('0.001').ok).toBe(false); // 四舍五入到 0
    expect(parseSellPriceInput('1000.01').ok).toBe(false);
    expect(parseSellPriceInput('x').ok).toBe(false);
    expect(parseSellPriceInput('').ok).toBe(false);
  });
});

describe('parseMarkupInput', () => {
  it('合法', () => {
    expect(parseMarkupInput('1.5')).toEqual({ ok: true, value: 1.5 });
    expect(parseMarkupInput('100')).toEqual({ ok: true, value: 100 });
    expect(parseMarkupInput('1.23456')).toEqual({ ok: true, value: 1.2346 });
  });
  it('不合法', () => {
    expect(parseMarkupInput('0').ok).toBe(false);
    expect(parseMarkupInput('100.1').ok).toBe(false);
    expect(parseMarkupInput('150%').ok).toBe(false);
    expect(parseMarkupInput('').ok).toBe(false);
  });
});

describe('parsePercentSetting', () => {
  it('变化保护阈值 / 最小变动', () => {
    expect(parsePercentSetting('50', 0, 100000)).toEqual({ ok: true, value: 0.5 });
    expect(parsePercentSetting('2', 0, 100)).toEqual({ ok: true, value: 0.02 });
    expect(parsePercentSetting('0', 0, 100)).toEqual({ ok: true, value: 0 });
  });
  it('最小变动不能到 100%', () => {
    expect(parsePercentSetting('100', 0, 100).ok).toBe(false);
    expect(parsePercentSetting('-1', 0, 100).ok).toBe(false);
    expect(parsePercentSetting('', 0, 100).ok).toBe(false);
  });
});

describe('设置页百分比输入', () => {
  it('变化保护阈值必须大于 0', () => {
    expect(parseThreshold('50')).toEqual({ ok: true, value: 0.5 });
    expect(parseThreshold('0').ok).toBe(false);
    expect(parseThreshold('0.001').ok).toBe(false);
    expect(parseThreshold('abc').ok).toBe(false);
  });
  it('最小变动 0% 可以，100% 不行', () => {
    expect(parseMinChange('2')).toEqual({ ok: true, value: 0.02 });
    expect(parseMinChange('0')).toEqual({ ok: true, value: 0 });
    expect(parseMinChange('99.99')).toEqual({ ok: true, value: 0.9999 });
    expect(parseMinChange('100').ok).toBe(false);
  });
});

describe('换算', () => {
  it('折合人民币', () => {
    expect(cnyPerUsd(0.3, 7.2)).toBeCloseTo(2.16, 10);
  });
  it('厂商分组倍率 = 售价 × 美元参考汇率 ÷ 站点汇率，保留 3 位小数', () => {
    expect(targetGroupRatio(0.3, 7.2, 7)).toBe(0.309);
    expect(targetGroupRatio(1, 7, 7)).toBe(1);
    expect(targetGroupRatio(0.5, 7.2, 7)).toBe(0.514);
  });
});

describe('parseGroupRatioInput / groupRatioToText', () => {
  it('小数输入，四舍五入到 3 位', () => {
    expect(parseGroupRatioInput('0.309')).toEqual({ ok: true, value: 0.309 });
    expect(parseGroupRatioInput(' 0.30857 ')).toEqual({ ok: true, value: 0.309 });
    expect(parseGroupRatioInput('1')).toEqual({ ok: true, value: 1 });
    expect(parseGroupRatioInput('.5')).toEqual({ ok: true, value: 0.5 });
    expect(parseGroupRatioInput('12.3456')).toEqual({ ok: true, value: 12.346 });
  });
  it('必须大于 0（取整后也算）', () => {
    expect(parseGroupRatioInput('0').ok).toBe(false);
    expect(parseGroupRatioInput('0.0004').ok).toBe(false);
    expect(parseGroupRatioInput('0.0005')).toEqual({ ok: true, value: 0.001 });
  });
  it('非数字写法拒绝', () => {
    expect(parseGroupRatioInput('').ok).toBe(false);
    expect(parseGroupRatioInput('abc').ok).toBe(false);
    expect(parseGroupRatioInput('-1').ok).toBe(false);
    expect(parseGroupRatioInput('1e3').ok).toBe(false);
    expect(parseGroupRatioInput('30%').ok).toBe(false);
  });
  it('倍率转输入框文字', () => {
    expect(groupRatioToText(0.30857)).toBe('0.309');
    expect(groupRatioToText(1)).toBe('1');
    expect(groupRatioToText(0.5)).toBe('0.5');
  });
  it('悬停原值提示', () => {
    expect(rawRatioTitle(0.30857)).toBe('原值 0.30857');
    expect(rawRatioTitle(null)).toBeUndefined();
  });
});

describe('currentPrice / signedChange', () => {
  it('固定看 sell_price，跟随看 target_price', () => {
    expect(currentPrice(item('synced', { mode: 'fixed', sell_price: 0.3, target_price: 0.3 }))).toBe(0.3);
    expect(currentPrice(item('synced', { mode: 'fixed', sell_price: null, target_price: null }))).toBeNull();
    expect(currentPrice(item('to_write', { mode: 'follow', sell_price: 0.1, target_price: 0.4 }))).toBe(0.4);
  });
  it('变化直接用后端给的带方向的值', () => {
    expect(signedChange({ change: 0.5 })).toBe(0.5);
    expect(signedChange({ change: -0.5 })).toBe(-0.5);
    expect(signedChange({ change: null })).toBeNull();
  });
});

describe('hasDraft', () => {
  it('固定模式有草稿倍率才算有未写入的修改', () => {
    expect(hasDraft({ mode: 'fixed', draft_ratio: 0.3 })).toBe(true);
    expect(hasDraft({ mode: 'fixed', draft_ratio: null })).toBe(false);
  });
  it('跟随模式没有草稿', () => {
    expect(hasDraft({ mode: 'follow', draft_ratio: 0.3 })).toBe(false);
  });
});

describe('待处理统计', () => {
  const list = [item('pending'), item('pending'), item('synced'), item('to_write'), item('unset'), item('no_channel')];
  it('只数待确认', () => {
    expect(attentionCount(list)).toBe(2);
    expect(attentionCount([])).toBe(0);
    expect(attentionSummary(list)).toEqual({ pending: 2 });
  });
  it('needsAttention', () => {
    expect(needsAttention('pending')).toBe(true);
    expect(needsAttention('to_write')).toBe(false);
    expect(needsAttention('no_channel')).toBe(false);
    expect(needsAttention('synced')).toBe(false);
  });
});

describe('canPreviewWrite', () => {
  it('有待写入的厂商可以预览', () => {
    expect(canPreviewWrite({ error: '', items: [item('to_write')], groups: groups(false) })).toBe(true);
  });
  it('只有分组有变化也可以预览', () => {
    expect(canPreviewWrite({ error: '', items: [item('synced')], groups: groups(true) })).toBe(true);
  });
  it('什么都没变不能预览', () => {
    expect(canPreviewWrite({ error: '', items: [item('synced'), item('pending')], groups: groups(false) })).toBe(false);
  });
  it('NewAPI 选项解析失败或没数据时不能预览', () => {
    expect(canPreviewWrite({ error: '解析失败', items: [item('to_write')], groups: groups(true) })).toBe(false);
    expect(canPreviewWrite(undefined)).toBe(false);
  });
});

describe('prettyJson', () => {
  it('缩进显示', () => {
    expect(prettyJson('{"a":1}')).toBe('{\n  "a": 1\n}');
  });
  it('不是 JSON 时原样返回，空串显示（空）', () => {
    expect(prettyJson('oops')).toBe('oops');
    expect(prettyJson('')).toBe('（空）');
  });
});
