import { DASH } from '@/lib/format';

/** 空值占位：灰色的「—」。 */
export function Dash() {
  return <span className="text-muted-foreground">{DASH}</span>;
}
