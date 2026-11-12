import { ChevronDown, ChevronRight } from 'lucide-react';
import { useState } from 'react';
import { Badge } from '@/components/ui/misc';
import { cn } from '@/lib/utils';
import { IndeterminateCheckbox } from './IndeterminateCheckbox';
import { OTHER_KEY, UNOWNED_KEY, type ModelGroup } from './useModelGroups';

const DEFAULT_OPEN_GROUPS = 3;

interface Props {
  groups: ModelGroup[];
  checked: ReadonlySet<string>;
  /** 搜索中时强制展开所有分组。 */
  expandAll: boolean;
  onToggleModel: (model: string) => void;
  onToggleGroup: (models: string[], select: boolean) => void;
}

export function ModelGroupList({ groups, checked, expandAll, onToggleModel, onToggleGroup }: Props) {
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});

  const isOpen = (key: string, index: number): boolean =>
    expandAll || (overrides[key] ?? index < DEFAULT_OPEN_GROUPS);

  const toggleOpen = (key: string, index: number) =>
    setOverrides((prev) => ({ ...prev, [key]: !isOpen(key, index) }));

  return (
    <div className="flex flex-col gap-1.5">
      {groups.map((group, index) => {
        const all = group.models.map((m) => m.model);
        const picked = all.filter((m) => checked.has(m)).length;
        const open = isOpen(group.key, index);
        const warn = group.key === UNOWNED_KEY;
        return (
          <section key={group.key} className="rounded border">
            <div className={cn('flex items-center gap-2 px-2 py-1', warn && 'bg-warning/10')}>
              <IndeterminateCheckbox
                aria-label={`全选 ${group.label}`}
                checked={picked === all.length}
                indeterminate={picked > 0 && picked < all.length}
                onChange={(e) => onToggleGroup(all, e.target.checked)}
              />
              <button
                type="button"
                className="flex flex-1 items-center gap-1.5 text-left text-base font-medium"
                onClick={() => toggleOpen(group.key, index)}
              >
                {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
                <span className={cn(warn && 'text-warning')}>{group.label}</span>
                {group.key === OTHER_KEY && (
                  <span className="text-xs font-normal text-muted-foreground">不属于当前厂商</span>
                )}
                <Badge tone={warn ? 'warning' : 'neutral'}>
                  已选 {picked} / {all.length}
                </Badge>
              </button>
            </div>
            {open && (
              <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 border-t px-2 py-1.5">
                {group.models.map((m) => (
                  <label key={m.model} className="flex min-w-0 items-center gap-1.5 text-base">
                    <input type="checkbox" checked={checked.has(m.model)} onChange={() => onToggleModel(m.model)} />
                    <span className="truncate" title={m.model}>
                      {m.model}
                    </span>
                    {m.by === 'prefix' && (
                      <span className="shrink-0 text-xs text-muted-foreground" title={`匹配前缀：${m.prefix}`}>
                        前缀
                      </span>
                    )}
                  </label>
                ))}
              </div>
            )}
          </section>
        );
      })}
    </div>
  );
}
