import { Plus, RefreshCw } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/misc';
import { splitList, uniq } from '@/lib/list';
import { ModelGroupList } from './ModelGroupList';
import { groupModels, useVendorMatches } from './useModelGroups';

export interface ModelPickerProps {
  open: boolean;
  title?: string; // 默认「选择模型」
  description?: string;
  /** 候选模型，一般是上游拉取结果；为空时只能看已选并手动添加。 */
  available: string[];
  /** 当前已选，作为初始勾选。 */
  selected: string[];
  /** 只显示这些厂商的分组（C 流程按厂商拆分时用）；不传显示全部。其余厂商的模型归入「其他厂商」组。 */
  vendorFilter?: string[];
  /** 打开时的初始勾选；默认 = selected。C 流程传「厂商配置 ∩ 可用」。 */
  initial?: string[];
  /** 允许手动输入不在候选里的模型名，默认 true。 */
  allowCustom?: boolean;
  /** 「从上游拉取」按钮；不传则不显示。返回新的候选列表。 */
  onFetch?: () => Promise<string[]>;
  confirmText?: string; // 默认「确定」
  onConfirm: (models: string[]) => void;
  onClose: () => void;
}

type TabKey = 'new' | 'existing' | 'removed';

const TAB_LABEL: Record<TabKey, string> = { new: '新获取', existing: '已选', removed: '上游已删除' };

function isTabKey(v: string): v is TabKey {
  return v === 'new' || v === 'existing' || v === 'removed';
}

export function ModelPickerDialog(props: ModelPickerProps) {
  const { open, title = '选择模型', description, onClose } = props;
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent size="3xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className={description ? undefined : 'sr-only'}>
            {description ?? title}
          </DialogDescription>
        </DialogHeader>
        {/* 仅在打开时挂载，关闭后草稿状态自然丢弃 */}
        <PickerBody {...props} />
      </DialogContent>
    </Dialog>
  );
}

function PickerBody({
  available,
  selected,
  vendorFilter,
  initial,
  allowCustom = true,
  onFetch,
  confirmText = '确定',
  onConfirm,
  onClose,
}: ModelPickerProps) {
  const [candidates, setCandidates] = useState<string[]>(() => uniq(available));
  const [extras, setExtras] = useState<string[]>([]);
  const [checked, setChecked] = useState<ReadonlySet<string>>(() => new Set(initial ?? selected));
  const [query, setQuery] = useState('');
  const [tab, setTab] = useState<TabKey>('new');
  const [customText, setCustomText] = useState('');
  const [fetching, setFetching] = useState(false);
  const [fetchError, setFetchError] = useState('');

  const hasUpstream = candidates.length > 0;

  const tabModels = useMemo<Record<TabKey, string[]>>(() => {
    const sel = new Set(selected);
    const cand = new Set(candidates);
    const ext = new Set(extras);
    if (!hasUpstream) return { new: [], existing: uniq([...selected, ...extras]), removed: [] };
    return {
      new: uniq([...candidates, ...extras]).filter((m) => !sel.has(m)),
      existing: candidates.filter((m) => sel.has(m)),
      removed: selected.filter((m) => !cand.has(m) && !ext.has(m)),
    };
  }, [candidates, extras, selected, hasUpstream]);

  const visibleTabs = useMemo<TabKey[]>(() => {
    const keys = (['new', 'existing', 'removed'] as const).filter((k) => tabModels[k].length > 0);
    return keys.length > 0 ? keys : ['existing'];
  }, [tabModels]);
  const activeTab: TabKey = visibleTabs.includes(tab) ? tab : (visibleTabs[0] ?? 'existing');

  const allModels = useMemo(() => uniq([...candidates, ...extras, ...selected]), [candidates, extras, selected]);
  const { matches, isLoading: matching, isError: matchFailed } = useVendorMatches(allModels);

  const needle = query.trim().toLowerCase();
  const filteredByTab = useMemo<Record<TabKey, string[]>>(() => {
    const f = (list: string[]) => (needle ? list.filter((m) => m.toLowerCase().includes(needle)) : list);
    return { new: f(tabModels.new), existing: f(tabModels.existing), removed: f(tabModels.removed) };
  }, [tabModels, needle]);
  const filtered = filteredByTab[activeTab];
  const groups = useMemo(() => groupModels(filtered, matches, vendorFilter), [filtered, matches, vendorFilter]);

  const toggleModel = (model: string) =>
    setChecked((prev) => {
      const next = new Set(prev);
      if (!next.delete(model)) next.add(model);
      return next;
    });

  const setMany = (models: readonly string[], select: boolean) =>
    setChecked((prev) => {
      const next = new Set(prev);
      for (const m of models) {
        if (select) next.add(m);
        else next.delete(m);
      }
      return next;
    });

  const addCustom = () => {
    const names = splitList(customText);
    if (names.length === 0) return;
    const known = new Set(allModels);
    setExtras((prev) => uniq([...prev, ...names.filter((n) => !known.has(n))]));
    setMany(names, true);
    setCustomText('');
  };

  const fetchUpstream = async () => {
    if (!onFetch) return;
    setFetching(true);
    setFetchError('');
    try {
      setCandidates(uniq(await onFetch()));
    } catch (e) {
      setFetchError(e instanceof Error ? e.message : '拉取失败');
    } finally {
      setFetching(false);
    }
  };

  const selectedSet = new Set(selected);
  const added = [...checked].filter((m) => !selectedSet.has(m)).length;
  const removed = selected.filter((m) => !checked.has(m)).length;

  return (
    <>
      <div className="flex items-center gap-2">
        <Input
          className="flex-1"
          placeholder="搜索模型（不区分大小写）"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {onFetch && (
          <Button variant="outline" disabled={fetching} onClick={() => void fetchUpstream()}>
            <RefreshCw className={fetching ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} />
            从上游拉取
          </Button>
        )}
      </div>
      {fetchError && <p className="text-sm text-destructive">{fetchError}</p>}
      {allowCustom && (
        <div className="flex items-center gap-2">
          <Input
            className="flex-1"
            placeholder="手动添加模型名，逗号或换行分隔，回车加入"
            value={customText}
            onChange={(e) => setCustomText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                addCustom();
              }
            }}
          />
          <Button variant="outline" disabled={customText.trim() === ''} onClick={addCustom}>
            <Plus className="h-3.5 w-3.5" />
            添加
          </Button>
        </div>
      )}
      <Tabs value={activeTab} onValueChange={(v) => isTabKey(v) && setTab(v)}>
        <TabsList>
          {visibleTabs.map((k) => (
            <TabsTrigger key={k} value={k}>
              {TAB_LABEL[k]}（{filteredByTab[k].length}）
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <DialogBody className="min-h-40 pr-1">
        {groups.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {needle ? '没有匹配的模型' : '这里还没有模型'}
          </p>
        ) : (
          <ModelGroupList
            groups={groups}
            checked={checked}
            expandAll={needle !== ''}
            onToggleModel={toggleModel}
            onToggleGroup={setMany}
          />
        )}
      </DialogBody>
      {(matching || matchFailed) && (
        <p className="text-xs text-muted-foreground">
          {matching ? '厂商匹配中…' : '厂商匹配失败，模型暂按「未归属」显示'}
        </p>
      )}
      <DialogFooter className="items-center justify-between">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>
            已选 {checked.size} 个（新增 +{added}，移除 -{removed}）
          </span>
          <Button variant="ghost" size="sm" disabled={filtered.length === 0} onClick={() => setMany(filtered, true)}>
            全选当前页签
          </Button>
          <Button variant="ghost" size="sm" disabled={filtered.length === 0} onClick={() => setMany(filtered, false)}>
            清空
          </Button>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button onClick={() => onConfirm([...checked])}>{confirmText}</Button>
        </div>
      </DialogFooter>
    </>
  );
}
