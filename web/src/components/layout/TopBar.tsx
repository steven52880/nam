import { useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, KeyRound, LogOut, Moon, Sun, User } from 'lucide-react';
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/endpoints';
import { useChannels } from '@/api/queries';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useActionMutation } from '@/hooks/useActionMutation';
import { useSession } from '@/hooks/useSession';
import { ApplyDialog } from '@/components/entities/ApplyDialog';
import { useTheme } from '@/hooks/useTheme';
import { ConnectionStatus } from './ConnectionStatus';
import { GlobalSearch } from './GlobalSearch';
import { LayoutSaveButton } from './LayoutSaveButton';
import { PasswordDialog } from './PasswordDialog';

export function TopBar() {
  const session = useSession();
  const { theme, toggle } = useTheme();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [pwOpen, setPwOpen] = useState(false);
  const [applyOpen, setApplyOpen] = useState(false);
  const channels = useChannels();
  const draftCount = channels.data?.draft_count ?? 0;

  const logout = useActionMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      qc.clear();
      navigate('/login', { replace: true });
    },
  });

  const afterPasswordChange = () => {
    qc.clear();
    navigate('/login', { replace: true });
  };

  return (
    <header className="flex h-11 shrink-0 items-center gap-3 border-b bg-card px-3">
      <GlobalSearch />
      <div className="flex-1" />
      <ConnectionStatus />
      <Button variant="outline" onClick={() => setApplyOpen(true)} title="查看并应用草稿">
        待应用草稿 {draftCount}
      </Button>
      <ApplyDialog open={applyOpen} onOpenChange={setApplyOpen} />
      <LayoutSaveButton />
      <Button variant="ghost" size="icon" onClick={toggle} title="切换主题">
        {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline">
            <User className="h-3.5 w-3.5" />
            {session.identity.username || '管理员'}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>{session.mode === 'embedded' ? '嵌入模式' : '独立模式'}</DropdownMenuLabel>
          {session.mode === 'embedded' ? (
            <DropdownMenuItem asChild>
              <a href="/">
                <ArrowLeft className="h-3.5 w-3.5" />返回 NewAPI
              </a>
            </DropdownMenuItem>
          ) : (
            <>
              <DropdownMenuItem onSelect={() => setPwOpen(true)}>
                <KeyRound className="h-3.5 w-3.5" />修改密码
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => logout.mutate()}>
                <LogOut className="h-3.5 w-3.5" />退出登录
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {session.mode === 'standalone' && (
        <PasswordDialog open={pwOpen} onOpenChange={setPwOpen} onChanged={afterPasswordChange} />
      )}
    </header>
  );
}
