import { describe, expect, it, vi } from 'vitest';
import { createLayoutStore, type LayoutDoc, type SlotSpec } from './store';

interface Demo {
  n: number;
}

const demoSpec: SlotSpec<Demo> = {
  id: 'table:demo.grid',
  version: 1,
  defaults: () => ({ n: 0 }),
  parse: (d) => {
    if (typeof d === 'object' && d !== null && typeof (d as Demo).n === 'number') return { n: (d as Demo).n };
    return null;
  },
};

const otherSpec: SlotSpec<Demo> = { ...demoSpec, id: 'table:audit.list' };

function ready(doc: LayoutDoc | null = null) {
  const s = createLayoutStore();
  s.loaded(doc);
  return s;
}

describe('createLayoutStore', () => {
  it('加载前 set 被忽略，status 为 loading', () => {
    const s = createLayoutStore();
    expect(s.status()).toBe('loading');
    s.set(demoSpec, { n: 5 });
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.isDirty()).toBe(false);
    s.loaded(null);
    expect(s.status()).toBe('ready');
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.snapshot().slots).toEqual({});
  });

  it('加载失败后 status 为 error，set 与 reset 被忽略', () => {
    const s = createLayoutStore();
    s.failed();
    expect(s.status()).toBe('error');
    s.set(demoSpec, { n: 5 });
    s.reset('table:demo.grid');
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.isDirty()).toBe(false);
  });

  it('读默认值；同一份数据多次 get 返回同一个引用', () => {
    const s = ready();
    const a = s.get(demoSpec);
    expect(a).toEqual({ n: 0 });
    expect(s.get(demoSpec)).toBe(a);
  });

  it('读已保存的数据；引用稳定', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    const a = s.get(demoSpec);
    expect(a).toEqual({ n: 3 });
    expect(s.get(demoSpec)).toBe(a);
  });

  it('改动一个槽位，另一个槽位的引用不变', () => {
    const s = ready({ v: 1, slots: { 'table:audit.list': { v: 1, data: { n: 1 } } } });
    const other = s.get(otherSpec);
    s.set(demoSpec, { n: 2 });
    expect(s.get(otherSpec)).toBe(other);
    expect(s.get(demoSpec)).toEqual({ n: 2 });
  });

  it('改动后变为未保存并通知订阅者', () => {
    const s = ready();
    const cb = vi.fn();
    s.subscribe(cb);
    s.set(demoSpec, { n: 2 });
    expect(s.isDirty()).toBe(true);
    expect(cb).toHaveBeenCalledTimes(1);
  });

  it('取消订阅后不再通知', () => {
    const s = ready();
    const cb = vi.fn();
    const off = s.subscribe(cb);
    off();
    s.set(demoSpec, { n: 2 });
    expect(cb).not.toHaveBeenCalled();
  });

  it('改回已保存的值后不再算未保存', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    s.set(demoSpec, { n: 4 });
    expect(s.isDirty()).toBe(true);
    s.set(demoSpec, { n: 3 });
    expect(s.isDirty()).toBe(false);
  });

  it('保存期间继续修改，markSaved(旧快照) 后仍为未保存', () => {
    const s = ready();
    s.set(demoSpec, { n: 1 });
    const snap = s.snapshot();
    s.set(demoSpec, { n: 2 });
    s.markSaved(snap);
    expect(s.isDirty()).toBe(true);
    expect(s.get(demoSpec)).toEqual({ n: 2 });
    s.markSaved(s.snapshot());
    expect(s.isDirty()).toBe(false);
  });

  it('保存后没有后续修改则不再未保存', () => {
    const s = ready();
    s.set(demoSpec, { n: 1 });
    s.markSaved(s.snapshot());
    expect(s.isDirty()).toBe(false);
  });

  it('恢复上次保存', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    s.set(demoSpec, { n: 9 });
    s.set(otherSpec, { n: 1 });
    s.revertToSaved();
    expect(s.get(demoSpec)).toEqual({ n: 3 });
    expect(s.get(otherSpec)).toEqual({ n: 0 });
    expect(s.isDirty()).toBe(false);
  });

  it('恢复时没改动的槽位引用不变', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    const a = s.get(demoSpec);
    s.set(otherSpec, { n: 1 });
    s.revertToSaved();
    expect(s.get(demoSpec)).toBe(a);
  });

  it('恢复默认：resetAll 清空全部当前值（含不认识的槽位），变为未保存', () => {
    const s = ready({
      v: 1,
      slots: { 'table:demo.grid': { v: 1, data: { n: 3 } }, 'x:unknown': { v: 1, data: 1 } },
    });
    s.resetAll();
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.snapshot().slots).toEqual({});
    expect(s.isDirty()).toBe(true);
    s.revertToSaved();
    expect(Object.keys(s.snapshot().slots).sort()).toEqual(['table:demo.grid', 'x:unknown']);
    expect(s.isDirty()).toBe(false);
  });

  it('reset(id) 只删除该槽位的当前值', () => {
    const s = ready({
      v: 1,
      slots: { 'table:demo.grid': { v: 1, data: { n: 3 } }, 'table:audit.list': { v: 1, data: { n: 4 } } },
    });
    s.reset('table:demo.grid');
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.get(otherSpec)).toEqual({ n: 4 });
    expect(s.isDirty()).toBe(true);
  });

  it('reset 一个本来就没有值的槽位：不算改动、不通知', () => {
    const s = ready();
    const cb = vi.fn();
    s.subscribe(cb);
    s.reset('table:demo.grid');
    expect(s.isDirty()).toBe(false);
    expect(cb).not.toHaveBeenCalled();
  });

  it('版本不符时返回默认值，且原始数据保留', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 99, data: { n: 3 } } } });
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.snapshot().slots['table:demo.grid']).toEqual({ v: 99, data: { n: 3 } });
    expect(s.isDirty()).toBe(false);
  });

  it('parse 返回 null 时返回默认值，且原始数据保留', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: 'bad' } } });
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.snapshot().slots['table:demo.grid']).toEqual({ v: 1, data: 'bad' });
    expect(s.isDirty()).toBe(false);
  });

  it('没访问过的槽位、不认识的槽位原样保留在快照里', () => {
    const doc: LayoutDoc = {
      v: 1,
      slots: { 'table:audit.list': { v: 1, data: { n: 4 } }, 'future:slot': { v: 7, data: { a: [1, 2] } } },
    };
    const s = ready(doc);
    s.set(demoSpec, { n: 1 });
    const snap = s.snapshot();
    expect(snap.v).toBe(1);
    expect(snap.slots['table:audit.list']).toEqual({ v: 1, data: { n: 4 } });
    expect(snap.slots['future:slot']).toEqual({ v: 7, data: { a: [1, 2] } });
    expect(snap.slots['table:demo.grid']).toEqual({ v: 1, data: { n: 1 } });
  });

  it('快照是拷贝，修改它不影响仓库', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    const snap = s.snapshot();
    (snap.slots['table:demo.grid'].data as Demo).n = 100;
    expect(s.get(demoSpec)).toEqual({ n: 3 });
  });

  it('对账（get 时补全）不产生未保存', () => {
    const reconciling: SlotSpec<Demo> = { ...demoSpec, parse: (d) => ({ n: ((d as Demo).n ?? 0) + 1 }) };
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    expect(s.get(reconciling)).toEqual({ n: 4 });
    expect(s.isDirty()).toBe(false);
    expect(s.snapshot().slots['table:demo.grid'].data).toEqual({ n: 3 });
  });

  it('文档版本不是 1：当作空文档加载并警告，保存时写成第 1 版', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const s = ready({ v: 2, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } } as unknown as LayoutDoc);
    expect(warn).toHaveBeenCalled();
    expect(s.get(demoSpec)).toEqual({ n: 0 });
    expect(s.snapshot().v).toBe(1);
    expect(s.snapshot().slots).toEqual({});
    warn.mockRestore();
  });

  it('loaded 会清掉之前的修改', () => {
    const s = ready();
    s.set(demoSpec, { n: 2 });
    s.loaded({ v: 1, slots: {} });
    expect(s.isDirty()).toBe(false);
    expect(s.get(demoSpec)).toEqual({ n: 0 });
  });

  it('set 相同的值不通知也不变脏', () => {
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { n: 3 } } } });
    const cb = vi.fn();
    s.subscribe(cb);
    s.set(demoSpec, { n: 3 });
    expect(cb).not.toHaveBeenCalled();
    expect(s.isDirty()).toBe(false);
  });

  it('键顺序不同的数据不算改动', () => {
    interface Two { a: number; b: number }
    const spec: SlotSpec<Two> = {
      id: 'table:demo.grid',
      version: 1,
      defaults: () => ({ a: 0, b: 0 }),
      parse: (d) => d as Two,
    };
    const s = ready({ v: 1, slots: { 'table:demo.grid': { v: 1, data: { a: 1, b: 2 } } } });
    s.set(spec, { b: 2, a: 1 });
    expect(s.isDirty()).toBe(false);
  });
});
