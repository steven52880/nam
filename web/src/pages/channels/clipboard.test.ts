import { describe, expect, it } from 'vitest';
import { copyColumnsOf, copyRowText, planPaste } from './clipboard';
import { makeChannel } from './testRows';

describe('copyColumnsOf', () => {
  it('只保留可复制的列，顺序跟随选区（换列后）', () => {
    expect(copyColumnsOf(['weight', 'status', 'priority'])).toEqual(['weight', 'status', 'priority']);
  });

  it('隐藏列不在选区里，不影响投影', () => {
    // 假设"标签"列被隐藏：选区列只有可见列
    expect(copyColumnsOf(['status', 'priority', 'weight'])).toEqual(['status', 'priority', 'weight']);
  });

  it('夹着只读列时去掉只读列', () => {
    expect(copyColumnsOf(['priority', 'balance', 'weight'])).toEqual(['priority', 'weight']);
  });
});

describe('copyRowText', () => {
  const row = makeChannel({ priority: 3, weight: 20, tag: 'fast', status: 1 });

  it('按选区顺序复制，跳过只读列', () => {
    expect(copyRowText(row, ['weight', 'name', 'priority'])).toBe('20\t3');
  });

  it('全是只读列时为空串', () => {
    expect(copyRowText(row, ['name', 'upstream'])).toBe('');
  });
});

describe('planPaste', () => {
  it('夹着只读列：第 i 个可复制列取第 i 段文本（修正错位）', () => {
    const plan = planPaste([makeChannel({ id: 5 })], ['priority', 'balance', 'weight'], '7\t30');
    expect(plan.changes).toEqual([{ channel_id: 5, fields: { priority: 7, weight: 30 }, ext: {} }]);
    expect(plan.skipped).toEqual(['balance']);
  });

  it('换列后复制再粘贴：复制的文本能按同样的列顺序粘回', () => {
    const src = makeChannel({ id: 1, priority: 4, weight: 12 });
    const columns = ['weight', 'priority'];
    const text = copyRowText(src, columns);
    const plan = planPaste([makeChannel({ id: 2 })], columns, text);
    expect(plan.changes[0].fields).toEqual({ weight: 12, priority: 4 });
  });

  it('只有一段文本：广播到所有可复制列', () => {
    const plan = planPaste([makeChannel({ id: 2 })], ['priority', 'weight'], '5');
    expect(plan.changes).toEqual([{ channel_id: 2, fields: { priority: 5, weight: 5 }, ext: {} }]);
  });

  it('广播时无效的列被跳过并记录', () => {
    const plan = planPaste([makeChannel({ id: 2 })], ['priority', 'status'], '5');
    expect(plan.changes[0].fields).toEqual({ priority: 5 });
    expect(plan.skipped).toEqual(['status']);
  });

  it('多行粘贴：选区每一行都应用同一行文本，每行一个修改', () => {
    const rows = [makeChannel({ id: 1 }), makeChannel({ id: 2 }), makeChannel({ id: 3 })];
    const plan = planPaste(rows, ['priority', 'weight'], '1\t2');
    expect(plan.changes.map((c) => c.channel_id)).toEqual([1, 2, 3]);
    expect(plan.changes.every((c) => c.fields.priority === 1 && c.fields.weight === 2)).toBe(true);
  });

  it('文本段数比列少：多出的列不处理', () => {
    const plan = planPaste([makeChannel({ id: 1 })], ['priority', 'weight', 'tag'], '1\t2');
    expect(plan.changes[0].fields).toEqual({ priority: 1, weight: 2 });
  });

  it('末尾换行被忽略', () => {
    const plan = planPaste([makeChannel({ id: 1 })], ['priority'], '9\r\n');
    expect(plan.changes[0].fields).toEqual({ priority: 9 });
  });

  it('负数 id（草稿新建渠道）原样作为 channel_id', () => {
    const plan = planPaste([makeChannel({ id: -3 })], ['weight'], '8');
    expect(plan.changes).toEqual([{ channel_id: -3, fields: { weight: 8 }, ext: {} }]);
  });

  it('没有任何有效内容时不产生修改', () => {
    const plan = planPaste([makeChannel({ id: 1 })], ['name', 'balance'], 'x');
    expect(plan.changes).toEqual([]);
    expect(plan.skipped).toEqual(['name', 'balance']);
  });

  it('同一个只读列只记录一次', () => {
    const plan = planPaste([makeChannel({ id: 1 }), makeChannel({ id: 2 })], ['name', 'priority'], '1\t2');
    expect(plan.skipped).toEqual(['name']);
  });
});
