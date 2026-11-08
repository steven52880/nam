import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { toast } from 'sonner';
import { api } from '@/api/endpoints';
import { errorText } from '@/api/errors';
import type { Upstream } from '@/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { CheckList } from '../wizard/StepTest';
import { Field } from '../UpstreamForm';

interface AdapterForm {
  url: string;
  method: string;
  headers: string;
  jsonPath: string;
  factor: string;
  announcementUrl: string;
}

interface RawBalance {
  url?: string;
  method?: string;
  headers?: Record<string, string>;
  json_path?: string;
  factor?: number | null;
}

interface RawConfig {
  balance?: RawBalance | null;
  announcement_url?: string;
}

function parseConfig(raw: string): AdapterForm {
  let cfg: RawConfig = {};
  try {
    cfg = raw.trim() ? (JSON.parse(raw) as RawConfig) : {};
  } catch {
    cfg = {};
  }
  const b = cfg.balance ?? {};
  return {
    url: b.url ?? '',
    method: b.method ?? 'GET',
    headers: Object.entries(b.headers ?? {}).map(([k, v]) => `${k}: ${v}`).join('\n'),
    jsonPath: b.json_path ?? '',
    factor: b.factor === null || b.factor === undefined ? '' : String(b.factor),
    announcementUrl: cfg.announcement_url ?? '',
  };
}

function headersOf(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split('\n')) {
    const at = line.indexOf(':');
    if (at > 0) out[line.slice(0, at).trim()] = line.slice(at + 1).trim();
  }
  return out;
}

function serialize(form: AdapterForm): string {
  const cfg: RawConfig = {};
  if (form.url.trim()) {
    cfg.balance = {
      url: form.url.trim(),
      method: form.method || 'GET',
      headers: headersOf(form.headers),
      json_path: form.jsonPath.trim(),
      factor: form.factor.trim() ? Number(form.factor) : null,
    };
  }
  if (form.announcementUrl.trim()) cfg.announcement_url = form.announcementUrl.trim();
  return Object.keys(cfg).length ? JSON.stringify(cfg) : '';
}

export function AdapterTab({ upstream }: { upstream: Upstream }) {
  const [form, setForm] = useState<AdapterForm>(() => parseConfig(upstream.adapter_config));
  const set = (patch: Partial<AdapterForm>) => setForm((cur) => ({ ...cur, ...patch }));
  const editable = upstream.type === 'generic';

  const save = useMutation({
    mutationFn: () => api.updateUpstream(upstream.id, { adapter_config: serialize(form) }),
    onSuccess: () => toast.success('适配器配置已保存'),
    onError: (e: Error) => toast.error(errorText(e)),
  });
  const test = useMutation({
    mutationFn: async () => {
      await api.updateUpstream(upstream.id, { adapter_config: serialize(form) });
      return api.testUpstream(upstream.id);
    },
    onError: (e: Error) => toast.error(errorText(e)),
  });

  if (!editable) {
    return (
      <div className="p-4 text-muted-foreground">
        只有 generic 类型的上游需要配置适配器；「{upstream.type}」使用内置适配器，无需配置。
      </div>
    );
  }
  const factorOk = form.factor.trim() === '' || Number(form.factor) > 0;
  return (
    <div className="grid gap-3 p-4">
      <Field label="余额请求地址"><Input value={form.url} onChange={(e) => set({ url: e.target.value })} placeholder="https://example.com/api/balance" /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="请求方法"><Input value={form.method} onChange={(e) => set({ method: e.target.value.toUpperCase() })} /></Field>
        <Field label="换算系数" hint={factorOk ? '余额乘以该系数得到美元；留空为 1' : '必须大于 0'}>
          <Input inputMode="decimal" value={form.factor} onChange={(e) => set({ factor: e.target.value })} />
        </Field>
      </div>
      <Field label="请求头" hint="每行一个，格式：名称: 值">
        <textarea
          className="min-h-[56px] w-full rounded border border-input bg-background px-2 py-1 text-base"
          value={form.headers}
          onChange={(e) => set({ headers: e.target.value })}
        />
      </Field>
      <Field label="JSON 路径" hint="例如 data.balance"><Input value={form.jsonPath} onChange={(e) => set({ jsonPath: e.target.value })} /></Field>
      <Field label="公告页面地址"><Input value={form.announcementUrl} onChange={(e) => set({ announcementUrl: e.target.value })} /></Field>
      <div className="flex gap-2">
        <Button disabled={!factorOk || save.isPending} onClick={() => save.mutate()}>保存配置</Button>
        <Button variant="outline" disabled={!factorOk || test.isPending} onClick={() => test.mutate()}>保存并测试</Button>
      </div>
      {(test.data || test.isPending) && <CheckList results={test.data ?? []} pending={test.isPending} />}
    </div>
  );
}
