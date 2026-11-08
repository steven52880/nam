import { ApiError } from './client';
import type { ValidationIssue } from './types';

function isIssue(v: unknown): v is ValidationIssue {
  return typeof v === 'object' && v !== null && 'message' in v && 'field' in v;
}

/** 从 400 响应的 data.issues 取出校验问题。 */
export function issuesOf(e: unknown): ValidationIssue[] {
  if (!(e instanceof ApiError)) return [];
  const data = e.data;
  if (typeof data !== 'object' || data === null || !('issues' in data)) return [];
  const raw = (data as { issues: unknown }).issues;
  return Array.isArray(raw) ? raw.filter(isIssue) : [];
}

export function issueText(issue: ValidationIssue): string {
  return issue.name ? `${issue.name}：${issue.message}` : issue.message;
}

/** 把错误整理成一行可读文字；有校验问题时逐条列出。 */
export function errorText(e: unknown): string {
  const issues = issuesOf(e);
  if (issues.length > 0) return issues.map(issueText).join('；');
  return e instanceof Error ? e.message : '请求失败';
}
