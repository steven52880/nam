import type { UpstreamToken } from '@/api/types';
import { StepModels } from './StepModels';

interface StepTokenModelsProps {
  upstreamId: number;
  tokens: UpstreamToken[];
  tokenIds: number[];
  value: Record<number, string[]>;
  onChange: (tokenId: number, models: string[]) => void;
}

/** 第 3 步：每个已选令牌一个区块，分别选择要开放的模型。 */
export function StepTokenModels({ upstreamId, tokens, tokenIds, value, onChange }: StepTokenModelsProps) {
  return (
    <div className="grid gap-3">
      <p className="text-sm text-muted-foreground">每个令牌单独选模型，默认填入令牌已保存的模型列表。一个令牌通常只对应一个厂商。</p>
      {tokenIds.map((id) => {
        const token = tokens.find((t) => t.id === id);
        const models = value[id] ?? [];
        return (
          <section key={id} className="grid gap-2 rounded border p-2">
            <h3 className="text-base font-medium">
              {token?.name ?? `令牌 ${id}`}
              <span className="ml-2 text-sm font-normal text-muted-foreground">
                {token?.group ? `分组 ${token.group} · ` : ''}{models.length} 个模型
              </span>
            </h3>
            <StepModels upstreamId={upstreamId} tokenId={id} value={models} onChange={(next) => onChange(id, next)} />
          </section>
        );
      })}
      {tokenIds.length === 0 && <div className="py-6 text-center text-muted-foreground">请先返回上一步选择令牌</div>}
    </div>
  );
}
