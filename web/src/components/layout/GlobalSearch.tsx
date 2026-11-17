import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useUpstreams } from '@/api/queries';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Search } from 'lucide-react';
import { ALL_NAV_ITEMS } from './nav';

export function GlobalSearch() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const upstreams = useUpstreams();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const go = (to: string) => {
    setOpen(false);
    navigate(to);
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex h-7 w-64 items-center gap-2 rounded border border-input bg-background px-2 text-base text-muted-foreground hover:bg-accent"
      >
        <Search className="h-3.5 w-3.5" />
        <span className="flex-1 text-left">搜索页面、上游…</span>
        <kbd className="rounded border px-1 text-xs">Ctrl K</kbd>
      </button>
      <CommandDialog open={open} onOpenChange={setOpen}>
        <CommandInput placeholder="输入页面或上游名称…" />
        <CommandList>
          <CommandEmpty>没有匹配的结果</CommandEmpty>
          <CommandGroup heading="页面">
            {ALL_NAV_ITEMS.map((item) => (
              <CommandItem key={item.to} value={`页面 ${item.label}`} onSelect={() => go(item.to)}>
                <item.icon className="h-3.5 w-3.5" />
                {item.label}
              </CommandItem>
            ))}
          </CommandGroup>
          <CommandGroup heading="上游">
            {(upstreams.data ?? []).map((u) => (
              <CommandItem
                key={u.id}
                value={`上游 ${u.name} ${u.base_url}`}
                onSelect={() => go(`/upstreams?open=${u.id}`)}
              >
                <span>{u.name}</span>
                <span className="truncate text-sm text-muted-foreground">{u.base_url}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
