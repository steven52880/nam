import type { ChannelModels, ModelRow } from '@/api/types';
import { formatNumber, formatPercent } from './format';

/** 偏离的比较基准：有上游分组倍率用它，否则用渠道里各模型推算倍率的中位数；都没有返回 null。 */
export interface DeviationBase {
  value: number;
  label: string;
}

export function deviationBase(
  detail: Pick<ChannelModels, 'upstream_group_ratio' | 'derived_median'>,
): DeviationBase | null {
  if (detail.upstream_group_ratio !== null) return { value: detail.upstream_group_ratio, label: '上游分组倍率' };
  if (detail.derived_median !== null) return { value: detail.derived_median, label: '渠道中位数' };
  return null;
}

/**
 * 偏离说明（悬停文字）：上游给这个模型的标价是原厂价的 X 倍，和基准 Y 不一致，偏离 Z%。
 * 模型自己没有推算倍率时返回空串。
 */
export function deviationHint(
  row: Pick<ModelRow, 'derived' | 'deviation'>,
  base: DeviationBase | null,
): string {
  if (row.derived === null) return '';
  const head = `上游给这个模型的标价是原厂价的 ${formatNumber(row.derived, 3, { trim: true })} 倍`;
  if (base === null) return head;
  const tail = `和${base.label} ${formatNumber(base.value, 3, { trim: true })} 不一致`;
  return row.deviation === null ? `${head}，${tail}` : `${head}，${tail}，偏离 ${formatPercent(row.deviation, { signed: true })}`;
}
