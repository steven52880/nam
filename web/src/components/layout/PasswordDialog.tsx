import { useState } from 'react';
import { api } from '@/api/endpoints';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/form/Field';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { useActionMutation } from '@/hooks/useActionMutation';

export function PasswordDialog({
  open,
  onOpenChange,
  onChanged,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onChanged: () => void;
}) {
  const [oldPw, setOldPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');

  const mutation = useActionMutation({
    mutationFn: () =>
      api.changePassword({ old_password: oldPw, new_password: newPw, confirm_password: confirmPw }),
    success: '密码已修改，所有会话已失效，请重新登录',
    onSuccess: () => {
      onOpenChange(false);
      onChanged();
    },
  });

  const tooShort = newPw.length > 0 && newPw.length < 8;
  const mismatch = confirmPw.length > 0 && newPw !== confirmPw;
  const valid = oldPw !== '' && newPw.length >= 8 && newPw === confirmPw;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>修改密码</DialogTitle>
        </DialogHeader>
        <form
          className="flex min-h-0 flex-1 flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (valid) mutation.mutate();
          }}
        >
          <DialogBody className="grid content-start gap-3">
            <Field label="当前密码">
              <Input type="password" autoFocus value={oldPw} onChange={(e) => setOldPw(e.target.value)} />
            </Field>
            <Field label="新密码（至少 8 位）">
              <Input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} />
              {tooShort && <span className="text-sm text-destructive">新密码至少 8 位</span>}
            </Field>
            <Field label="确认新密码">
              <Input type="password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} />
              {mismatch && <span className="text-sm text-destructive">两次输入不一致</span>}
            </Field>
            <p className="text-sm text-muted-foreground">修改后，其它会话全部失效。</p>
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" disabled={!valid || mutation.isPending}>
              确认修改
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
