import { describe, expect, it } from 'vitest';
import type { ValidationIssue } from '@/api/types';
import { groupIssues } from './issues';

const issue = (channel_id: number, name: string, message: string): ValidationIssue => ({ channel_id, name, field: 'models', message });

describe('groupIssues', () => {
  it('没有问题返回空数组', () => {
    expect(groupIssues([])).toEqual([]);
  });
  it('同一渠道的多条问题归到一起，保持首次出现的顺序', () => {
    const list = [issue(3, 'a', 'x'), issue(5, 'b', 'y'), issue(3, 'a', 'z')];
    expect(groupIssues(list)).toEqual([
      { channelId: 3, name: 'a', messages: ['x', 'z'] },
      { channelId: 5, name: 'b', messages: ['y'] },
    ]);
  });
});
