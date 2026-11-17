/**
 * 筛选后只能看到部分组时，拖动只调整这些组之间的相对位置，其它组的已保存位置不变。
 *
 * fullOrder：未筛选时全部组的当前生效顺序（不是槽位里保存的顺序）。
 * visibleNewOrder：可见组拖动后的新顺序。
 *
 * 1. base = fullOrder，再追加 visibleNewOrder 里 fullOrder 没有的键。
 * 2. 找出 base 中属于 visibleNewOrder 的位置，按顺序填入 visibleNewOrder；其它位置不动。
 *
 * 例：全部为 A/B/C，筛选后只见 A/C，拖成 C/A → C/B/A。
 */
export function mergeVisibleOrder(fullOrder: string[], visibleNewOrder: string[]): string[] {
  const visible: string[] = [];
  const visibleSet = new Set<string>();
  for (const k of visibleNewOrder) {
    if (!visibleSet.has(k)) {
      visibleSet.add(k);
      visible.push(k);
    }
  }

  const base: string[] = [];
  const inBase = new Set<string>();
  for (const k of [...fullOrder, ...visible]) {
    if (!inBase.has(k)) {
      inBase.add(k);
      base.push(k);
    }
  }

  let next = 0;
  return base.map((k) => (visibleSet.has(k) ? visible[next++] : k));
}
