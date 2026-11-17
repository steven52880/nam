import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/misc';

export type ModelCountVariant =
  /** 「N 个模型」，悬停显示完整列表（渠道表）。 */
  | 'tooltip'
  /** 「N 个模型」，纯文字（模型与价格页的计数）。 */
  | 'text'
  /** 「N 个」（向导预览表）。 */
  | 'count'
  /** 「N 个：前三个…」（厂商表）。 */
  | 'preview'
  /** 「已选 N 个模型」（令牌模型步骤）。 */
  | 'selected'
  /** 「N 个模型偏离」（推算折扣）。 */
  | 'deviated';

interface ModelCountCellProps {
  /** 模型名列表；只有个数时传 count。 */
  models?: readonly string[];
  count?: number;
  variant?: ModelCountVariant;
}

/** 模型数量文字。各处原有写法不一致，用 variant 保留各自的文案。 */
export function ModelCountCell({ models, count, variant = 'tooltip' }: ModelCountCellProps) {
  const n = count ?? models?.length ?? 0;
  switch (variant) {
    case 'text':
      return <>{n} 个模型</>;
    case 'count':
      return <>{n} 个</>;
    case 'preview': {
      const list = models ?? [];
      return <>{n} 个：{list.slice(0, 3).join('，')}{n > 3 && '…'}</>;
    }
    case 'selected':
      return <>{`已选 ${n} 个模型`}</>;
    case 'deviated':
      return <>{`${n} 个模型偏离`}</>;
    default:
      return (
        <Tooltip>
          <TooltipTrigger asChild><span>{n} 个模型</span></TooltipTrigger>
          <TooltipContent className="max-h-60 overflow-y-auto">{(models ?? []).join('，') || '无'}</TooltipContent>
        </Tooltip>
      );
  }
}
