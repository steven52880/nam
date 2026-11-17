import type { GridRange } from './types';

export function GridHintBar({ range }: { range: GridRange | null }) {
  const rows = range?.rowIds.length ?? 0;
  const cols = range?.columnIds.length ?? 0;
  return (
    <div className="flex h-6 shrink-0 items-center justify-between border-t bg-muted px-2 text-sm text-muted-foreground">
      <span>{range ? `已选 ${rows} 行 × ${cols} 列（${rows * cols} 格）` : '未选择单元格'}</span>
      <span>拖动框选 · Shift+点击/方向键扩展 · 双击/Enter/F2 编辑 · Ctrl+C / Ctrl+V · Esc 清除</span>
    </div>
  );
}
