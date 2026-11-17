import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import { keys } from '@/api/keys';
import { groupRatioOf, useTokenModelsFetch, useUpstreamGroups, useVendors } from '@/api/queries';
import type { UpstreamToken } from '@/api/types';
import { ModelCountCell } from '@/components/cells/ModelCountCell';
import { ModelPickerDialog } from '@/components/model-picker';
import { useVendorMatches } from '@/components/model-picker/useModelGroups';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/misc';
import { formatNumber } from '@/lib/format';
import { parseStringArray, uniq } from '@/lib/list';

interface StepTokenModelsProps {
  upstreamId: number;
  tokens: UpstreamToken[];
  tokenIds: number[];
  value: Record<number, string[]>;
  onChange: (tokenId: number, models: string[]) => void;
}


/** 第 3 步：进入时自动从上游拉取每个令牌的模型，按厂商前缀预选；跨厂商的令牌会拆成多个渠道。 */
export function StepTokenModels({ upstreamId, tokens, tokenIds, value, onChange }: StepTokenModelsProps) {
  const qc = useQueryClient();
  const [pickerId, setPickerId] = useState<number | null>(null);
  const vendors = useVendors();
  const groups = useUpstreamGroups(upstreamId);
  const fetches = useTokenModelsFetch(upstreamId, tokenIds);

  const fetchedKey = fetches.map((f) => (f.data ? f.data.length : -1)).join(',');
  const allModels = useMemo(
    () => uniq([...fetches.flatMap((f) => f.data ?? []), ...tokenIds.flatMap((id) => value[id] ?? [])]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [fetchedKey, tokenIds, value],
  );
  const { matches, isError: matchFailed } = useVendorMatches(allModels);

  // 预选：厂商显式模型 ∩ 可用；厂商没有显式模型时取全部前缀匹配的模型。已有值（用户调整过）不覆盖。
  useEffect(() => {
    if (!vendors.data) return;
    const explicitVendors = new Set(vendors.data.filter((v) => parseStringArray(v.models).length > 0).map((v) => v.name));
    tokenIds.forEach((id, index) => {
      const fetched = fetches[index]?.data;
      if (value[id] !== undefined || !fetched) return;
      if (!fetched.every((m) => matches.has(m))) return;
      const picked = fetched.filter((m) => {
        const hit = matches.get(m);
        if (!hit || hit.vendor === '') return false;
        return hit.by === 'explicit' || (hit.by === 'prefix' && !explicitVendors.has(hit.vendor));
      });
      onChange(id, picked);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vendors.data, matches, fetchedKey, tokenIds, value]);

  const ratioOf = (group: string) => groupRatioOf(groups.data, group);

  return (
    <div className="grid gap-3">
      <p className="text-sm text-muted-foreground">
        自动从上游拉取每个令牌可用的模型，并按「设置 → 厂商分组」的模型名前缀归属厂商；一个令牌跨多个厂商时会拆成多个渠道。调整只对本次创建生效，不会写回令牌。
      </p>
      {matchFailed && <div className="rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">厂商匹配失败，无法自动预选，请手动调整模型。</div>}
      {tokenIds.map((id, index) => {
        const token = tokens.find((t) => t.id === id);
        const fetch = fetches[index];
        const fetched = fetch?.data ?? [];
        const selected = value[id] ?? [];
        const vendorOf = (m: string) => matches.get(m)?.vendor ?? '';
        const vendorNames = uniq([...fetched, ...selected].map(vendorOf).filter((v) => v !== ''));
        const unowned = selected.filter((m) => vendorOf(m) === '');
        const splitCount = new Set(selected.map(vendorOf).filter((v) => v !== '')).size;
        const ratio = token ? ratioOf(token.group) : null;
        return (
          <section key={id} className="grid gap-2 rounded border p-2">
            <div className="flex items-center gap-2">
              <h3 className="text-base font-medium">
                {token?.name ?? `令牌 ${id}`}
                <span className="ml-2 text-sm font-normal text-muted-foreground">
                  {token?.group ? `分组 ${token.group}` : '无分组'}
                  {ratio !== null && `（倍率 ${formatNumber(ratio, 3)}）`}
                  {' · '}<ModelCountCell count={selected.length} variant="selected" />
                </span>
              </h3>
              <Button
                size="sm"
                variant="outline"
                className="ml-auto"
                disabled={fetch?.isFetching}
                onClick={() => setPickerId(id)}
              >
                调整模型
              </Button>
            </div>
            {fetch?.isFetching && <div className="text-sm text-muted-foreground">正在从上游拉取模型…</div>}
            {fetch?.isError && !fetch.isFetching && (
              <div className="flex items-center gap-2 rounded bg-destructive/10 px-2 py-1 text-sm text-destructive">
                <span className="flex-1">拉取失败：{errorText(fetch.error)}（仍可点「调整模型」手动输入）</span>
                <Button size="sm" variant="outline" onClick={() => void fetch.refetch()}>重试</Button>
              </div>
            )}
            {vendorNames.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {vendorNames.map((name) => (
                  <Badge key={name} tone="accent">
                    {name} 已选 {selected.filter((m) => vendorOf(m) === name).length} / 可用 {fetched.filter((m) => vendorOf(m) === name).length}
                  </Badge>
                ))}
              </div>
            )}
            {splitCount > 1 && <div className="text-sm text-muted-foreground">将拆成 {splitCount} 个渠道（每个厂商一个）。</div>}
            {unowned.length > 0 && (
              <div className="rounded bg-warning/15 px-2 py-1 text-sm text-warning">
                {unowned.length} 个已选模型不属于任何厂商，生成渠道时会被忽略：{unowned.slice(0, 6).join('，')}{unowned.length > 6 && '…'}
              </div>
            )}
            {selected.length === 0 && !fetch?.isFetching && !fetch?.isError && (
              <div className="text-sm text-warning">还没有选中任何模型，请点「调整模型」选择。</div>
            )}
            {pickerId === id && (
              <ModelPickerDialog
                open
                title={`选择模型：${token?.name ?? `令牌 ${id}`}`}
                available={fetched}
                selected={selected}
                onFetch={async () => {
                  const res = await api.fetchUpstreamModels(upstreamId, id);
                  qc.setQueryData(keys.upstreams.tokenModelsFetch(upstreamId, id), res.models);
                  return res.models;
                }}
                onConfirm={(models) => {
                  onChange(id, models);
                  setPickerId(null);
                }}
                onClose={() => setPickerId(null)}
              />
            )}
          </section>
        );
      })}
      {tokenIds.length === 0 && <div className="py-6 text-center text-muted-foreground">请先返回上一步选择令牌</div>}
    </div>
  );
}
