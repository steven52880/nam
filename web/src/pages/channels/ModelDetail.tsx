import type { ChannelRow } from '@/api/types';
import { formatNumber } from '@/lib/format';

const DASH_COLUMNS = ['原厂价（输入 / 输出）', '上游分组折扣覆盖', '成本（元 / 百万）', '请求数', '成功率', '首字延迟', '吐字速度'];

function Dash() {
  return <span className="text-muted-foreground">—</span>;
}

export function ModelDetail({ row }: { row: ChannelRow | null }) {
  if (!row) {
    return (
      <div className="flex h-full items-center justify-center rounded border bg-card text-muted-foreground">
        点击上表中的渠道，查看它的模型明细
      </div>
    );
  }
  return (
    <div className="flex h-full min-h-0 flex-col rounded border bg-card">
      <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-0.5 border-b bg-muted/50 px-3 py-1 text-sm">
        <span className="font-medium">{row.name}</span>
        <span className="text-muted-foreground">厂商：{row.vendor || '—'}</span>
        <span className="text-muted-foreground">上游分组折扣：{formatNumber(row.discount, 3)}</span>
        <span className="text-muted-foreground">上游货币汇率：{formatNumber(row.currency_rate)}</span>
        <span className="text-muted-foreground">测试模型：{row.test_model || '—'}</span>
        <span className="ml-auto text-xs text-muted-foreground">价格与统计列将在后续里程碑提供</span>
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-max min-w-full border-separate border-spacing-0 text-base">
          <thead className="sticky top-0 bg-muted">
            <tr>
              <th className="h-7 border-b border-r px-2 text-left text-sm font-medium text-muted-foreground">模型名</th>
              {DASH_COLUMNS.map((h) => (
                <th key={h} className="h-7 whitespace-nowrap border-b border-r px-2 text-right text-sm font-medium text-muted-foreground">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {row.models.map((model) => (
              <tr key={model} className="hover:bg-accent/40">
                <td className="h-7 border-b border-r px-2">{model}</td>
                {DASH_COLUMNS.map((h) => <td key={h} className="border-b border-r px-2 text-right"><Dash /></td>)}
              </tr>
            ))}
            {row.models.length === 0 && (
              <tr><td colSpan={DASH_COLUMNS.length + 1} className="h-12 text-center text-muted-foreground">该渠道没有模型</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
