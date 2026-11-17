import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { api } from '@/api/endpoints';
import { keys } from '@/api/keys';
import { useSettings } from '@/api/queries';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DescRow } from '@/components/ui/section';
import { useActionMutation } from '@/hooks/useActionMutation';

/** 手动缓存率测试所需的 NewAPI 令牌与地址。令牌只写不读，界面只显示 hint。 */
export function CacheTestSection() {
  const qc = useQueryClient();
  const settings = useSettings();
  const [token, setToken] = useState('');
  const [url, setUrl] = useState('');
  const savedUrl = settings.data?.newapi_url ?? '';
  const hint = settings.data?.newapi_test_token_hint ?? '';

  useEffect(() => {
    setUrl(savedUrl);
  }, [savedUrl]);

  const save = useActionMutation({
    mutationFn: (body: Record<string, string>) => api.saveSettings(body),
    success: (_, body) => (body.newapi_test_token === '' ? '测试令牌已清除' : '缓存测试设置已保存'),
    onSuccess: (data) => {
      qc.setQueryData(keys.settings(), data);
      setToken('');
    },
  });

  const urlTrimmed = url.trim();
  const urlOk = urlTrimmed === '' || /^https?:\/\/\S+$/i.test(urlTrimmed);
  const dirty = token.trim() !== '' || urlTrimmed !== savedUrl;

  const onSave = () => {
    const body: Record<string, string> = { newapi_url: urlTrimmed };
    if (token.trim() !== '') body.newapi_test_token = token.trim();
    save.mutate(body);
  };

  return (
    <>
      <div className="text-sm text-muted-foreground">当前缓存测试默认直连上游，以下设置仅用于“经 NewAPI”模式（预留）</div>
      <DescRow label="NewAPI 测试令牌">
        <div className="flex items-center gap-2">
          <Input
            type="password"
            autoComplete="new-password"
            className="max-w-sm"
            placeholder={hint ? `当前：${hint}（留空则不修改）` : '尚未配置，粘贴管理员账号名下的令牌'}
            value={token}
            onChange={(e) => setToken(e.target.value)}
          />
          <Button
            variant="outline"
            disabled={!hint || save.isPending}
            onClick={() => save.mutate({ newapi_test_token: '' })}
          >
            清除
          </Button>
        </div>
      </DescRow>
      <DescRow label="NewAPI 地址">
        <Input
          className="max-w-sm"
          placeholder="留空使用默认：独立模式为 NAM_NEWAPI_URL，嵌入模式为 http://127.0.0.1:3000"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
        {!urlOk && <div className="mt-1 text-sm text-destructive">地址必须以 http:// 或 https:// 开头</div>}
      </DescRow>
      <div className="flex items-center gap-3">
        <Button disabled={!dirty || !urlOk || save.isPending || !settings.data} onClick={onSave}>保存</Button>
        <span className="text-sm text-muted-foreground">
          令牌需属于管理员账号；测试会经 NewAPI 真实调用并计费，令牌加密保存，界面不会回显。
        </span>
      </div>
    </>
  );
}
