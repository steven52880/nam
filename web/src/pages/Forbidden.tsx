import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';

export default function ForbiddenPage() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3">
      <div className="text-2xl font-semibold">无权限</div>
      <p className="text-muted-foreground">nam 只允许 root 用户访问。请使用 root 账号登录 NewAPI 后再试。</p>
      <Button asChild variant="outline">
        <a href="/">返回 NewAPI</a>
      </Button>
    </div>
  );
}

export function NotFoundPage() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-8">
      <div className="text-2xl font-semibold">页面不存在</div>
      <p className="text-muted-foreground">检查地址是否正确，或回到总览。</p>
      <Button asChild variant="outline">
        <Link to="/">回到总览</Link>
      </Button>
    </div>
  );
}
