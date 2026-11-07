import { PageHeader } from '@/components/layout/PageHeader';

export function PlaceholderPage({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex flex-col gap-3 p-4">
      <PageHeader title={title} description={description} />
      <div className="flex h-40 items-center justify-center rounded border border-dashed text-muted-foreground">
        此页面将在后续里程碑实现
      </div>
    </div>
  );
}
