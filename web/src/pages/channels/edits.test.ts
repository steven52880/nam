import { describe, expect, it } from 'vitest';
import { copyText, parseCell, validateCell } from './edits';
import { makeChannel } from './testRows';

describe('上游分组倍率的编辑', () => {
  it('输入数字 = 手填，只带 discount', () => {
    expect(parseCell('discount', '0.35')).toEqual({ patch: { fields: {}, ext: { discount: 0.35 } } });
    expect(parseCell('discount', ' 1 ', makeChannel({ discount_source: 'auto' }))).toEqual({
      patch: { fields: {}, ext: { discount: 1 } },
    });
  });
  it('当前是手填时清空 = 回到自动，不带 discount', () => {
    const r = parseCell('discount', '', makeChannel({ discount_source: 'manual' }));
    expect(r).toEqual({ patch: { fields: {}, ext: { discount_source: 'auto' } } });
    expect(r).not.toHaveProperty('patch.ext.discount');
  });
  it('当前不是手填时清空报错', () => {
    for (const source of ['auto', 'group', 'none'] as const) {
      expect('error' in parseCell('discount', '', makeChannel({ discount_source: source }))).toBe(true);
    }
    expect('error' in parseCell('discount', '')).toBe(true);
  });
  it('0、负数、非数字报错', () => {
    expect(validateCell('discount', '0')).toBe('上游分组倍率必须大于 0');
    expect(validateCell('discount', '-1')).toBe('上游分组倍率必须大于 0');
    expect(validateCell('discount', 'abc')).toBe('上游分组倍率必须大于 0');
    expect(validateCell('discount', '0.5')).toBeNull();
  });
  it('复制文本：有倍率显示数字，null 为空串', () => {
    expect(copyText(makeChannel({ discount: 0.35 }), 'discount')).toBe('0.35');
    expect(copyText(makeChannel({ discount: null, discount_source: 'none' }), 'discount')).toBe('');
  });
});
