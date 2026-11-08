import { useMutation, useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { Vendor } from '@/api/types';
import { ChipEditor } from '@/components/ChipEditor';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/input';
import { Badge } from '@/components/ui/misc';
import { parseStringArray, splitList, uniq } from '@/lib/list';

export function vendorModels(vendor: Vendor): string[] {
  return parseStringArray(vendor.models);
}

/** 模型是否属于某个厂商（精确名称或可选正则）。 */
export function vendorOf(model: string, vendors: Vendor[]): Vendor | null {
  for (const v of vendors) {
    if (vendorModels(v).includes(model)) return v;
    if (v.pattern) {
      try {
        if (new RegExp(v.pattern).test(model)) return v;
      } catch {
        continue;
      }
    }
  }
  return null;
}

interface StepModelsProps {
  upstreamId: number;
  firstTokenId: number | null;
  value: string[];
  onChange: (models: string[]) => void;
}

export function StepModels({ upstreamId, firstTokenId, value, onChange }: StepModelsProps) {
  const vendors = useQuery({ queryKey: ['vendors'], queryFn: api.vendors });
  const list = vendors.data ?? [];

  const fetchModels = useMutation({
    mutationFn: () => api.fetchUpstreamModels(upstreamId, firstTokenId ?? 0),
    onSuccess: (res) => {
      onChange(uniq([...value, ...res.models]));
      toast.success(`已从上游拉取 ${res.models.length} 个模型`);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  const hasAll = (names: string[]) => names.length > 0 && names.every((m) => value.includes(m));
  const toggleVendor = (vendor: Vendor) => {
    const names = vendorModels(vendor);
    onChange(hasAll(names) ? value.filter((m) => !names.includes(m)) : uniq([...value, ...names]));
  };
  const unassigned = value.filter((m) => !vendorOf(m, list));

  return (
    <div className="grid gap-3">
      <div>
        <div className="mb-1 text-sm text-muted-foreground">厂商模板（点击一键勾选 / 取消该厂商的全部模型）</div>
        <div className="flex flex-wrap gap-1.5">
          {list.map((v) => {
            const names = vendorModels(v);
            const checked = hasAll(names);
            return (
              <Button key={v.id} size="sm" variant={checked ? 'default' : 'outline'} onClick={() => toggleVendor(v)}>
                {v.name}（{names.length}）{v.channel_type === 24 && ' · Gemini 原生'}
              </Button>
            );
          })}
          {vendors.isSuccess && list.length === 0 && <span className="text-sm text-muted-foreground">还没有厂商，请先到「设置 → 厂商分组」添加</span>}
        </div>
      </div>
      <div className="grid gap-1">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">手动补充模型（逗号或换行分隔）</span>
          <Button
            size="sm"
            variant="outline"
            disabled={firstTokenId === null || fetchModels.isPending}
            onClick={() => fetchModels.mutate()}
            title="用已选的第一个令牌请求上游 /v1/models"
          >
            {fetchModels.isPending ? '拉取中…' : '从上游拉取'}
          </Button>
        </div>
        <Textarea
          placeholder="粘贴模型名后点击空白处添加"
          onBlur={(e) => {
            const items = splitList(e.target.value);
            if (items.length > 0) onChange(uniq([...value, ...items]));
            e.target.value = '';
          }}
        />
      </div>
      <div className="grid gap-1">
        <div className="text-sm text-muted-foreground">已选 {value.length} 个模型（黄色为未归属任何厂商）</div>
        <ChipEditor value={value} onChange={onChange} isWarn={(m) => !vendorOf(m, list)} />
      </div>
      {unassigned.length > 0 && (
        <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">
          <Badge tone="warning">提示</Badge> 有 {unassigned.length} 个模型不属于任何厂商，生成渠道时会被忽略：{unassigned.slice(0, 6).join('，')}
          {unassigned.length > 6 && '…'}
        </div>
      )}
    </div>
  );
}
