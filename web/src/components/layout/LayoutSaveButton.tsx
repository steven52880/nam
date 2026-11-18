import { AlertTriangle, Check, ChevronDown, Loader2, Save } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { MenuItems, type MenuItem } from '@/components/ui/menu-items';
import { useLayoutControls } from '@/lib/layout/LayoutProvider';
import { cn } from '@/lib/utils';

/**
 * 顶栏的布局保存按钮（分体按钮）：主按钮按状态显示，右侧箭头打开"恢复"菜单。
 * 主按钮宽度固定，状态切换时顶栏其它元素不会跳动。
 */
export function LayoutSaveButton() {
  const { status, dirty, saving, save, retry, revertToSaved, resetAll } = useLayoutControls();

  const ready = status === 'ready';
  const idleHint = status === 'loading' ? '布局加载中' : status === 'error' ? '布局加载失败，请先重试' : saving ? '正在保存，请稍候' : undefined;
  const menuLocked = !ready || saving;

  // 主按钮：文字、图标、样式、点击行为
  let label = '布局已保存';
  let icon = <Check className="h-4 w-4" />;
  let variant: 'default' | 'outline' | 'destructive' = 'outline';
  let tone = 'text-muted-foreground';
  let disabled = true;
  let onClick: (() => void) | undefined;
  let title = '当前布局已保存到服务器';

  if (status === 'loading') {
    label = '布局加载中';
    icon = <Loader2 className="h-4 w-4 animate-spin" />;
    title = '正在从服务器读取你保存的布局';
  } else if (status === 'error') {
    label = '布局加载失败，点击重试';
    icon = <AlertTriangle className="h-4 w-4" />;
    variant = 'destructive';
    tone = '';
    disabled = false;
    onClick = retry;
    title = '读取已保存的布局失败，点击重新读取；读取成功前不能调整布局';
  } else if (saving) {
    label = '保存中…';
    icon = <Loader2 className="h-4 w-4 animate-spin" />;
    title = '正在保存布局';
  } else if (dirty) {
    label = '保存布局';
    icon = <Save className="h-4 w-4" />;
    variant = 'default';
    tone = '';
    disabled = false;
    onClick = save;
    title = '有未保存的布局调整，点击保存到服务器';
  }

  const showDot = dirty && ready && !saving;

  const items: MenuItem[] = [
    {
      key: 'revert',
      label: '恢复上次保存',
      onSelect: revertToSaved,
      disabled: menuLocked || !dirty,
      title: idleHint ?? (dirty ? '放弃未保存的调整，回到上次保存的布局' : '没有未保存的调整'),
    },
    {
      key: 'reset',
      label: '恢复默认布局',
      onSelect: resetAll,
      disabled: menuLocked,
      title: idleHint ?? '所有表格、分栏、分组方式回到默认；恢复后需要再点“保存布局”才会写到服务器',
    },
  ];

  return (
    <div className="flex shrink-0 items-center" title={title}>
      <Button
        variant={variant}
        disabled={disabled}
        onClick={onClick}
        aria-label={showDot ? `${label}（有未保存的调整）` : label}
        className={cn('relative w-7 rounded-r-none px-0', tone)}
      >
        {icon}
        {showDot && (
          <span
            aria-hidden="true"
            className="absolute right-1 top-1 h-1.5 w-1.5 rounded-full bg-warning ring-1 ring-primary-foreground/70"
          />
        )}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant={variant}
            className={cn(
              'w-5 rounded-l-none px-0',
              variant === 'default' && 'border-l border-primary-foreground/30',
              variant === 'destructive' && 'border-l border-destructive-foreground/30',
              variant === 'outline' && 'border-l-0',
            )}
            title="布局菜单"
            aria-label="布局菜单"
          >
            <ChevronDown className="h-3 w-3" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <MenuItems kind="dropdown" items={items} />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
