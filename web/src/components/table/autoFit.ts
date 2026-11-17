/** 最多测量多少个单元格。 */
const MAX_SAMPLES = 500;

function attrSelector(columnId: string): string {
  return `[data-col-id="${columnId.replace(/["\\]/g, '\\$&')}"]`;
}

function frame(el: Element): number {
  const cs = getComputedStyle(el);
  return (
    (parseFloat(cs.paddingLeft) || 0) +
    (parseFloat(cs.paddingRight) || 0) +
    (parseFloat(cs.borderLeftWidth) || 0) +
    (parseFloat(cs.borderRightWidth) || 0)
  );
}

/**
 * 自动列宽：在 container 里临时放一个隐藏的测量层，把这一列已渲染的 data-cell-content 和表头内容
 * 逐个克隆进去，克隆体去掉 truncate、宽度设为 max-content，取最宽的一个，加上单元格左右内边距和边框。
 * 结果限制在 [minPx, maxPx]。能变宽也能变窄，重复调用结果不变；折叠组里没渲染的行不参与。
 * container 必须是定位元素（relative），测量层才会落在它里面。
 */
export function autoFit(container: HTMLElement, columnId: string, minPx: number, maxPx: number): number {
  const sel = attrSelector(columnId);
  const cells = Array.from(container.querySelectorAll<HTMLElement>(`td${sel} > [data-cell-content]`)).slice(
    0,
    MAX_SAMPLES,
  );
  const headers = Array.from(container.querySelectorAll<HTMLElement>(`th${sel} [data-header-content]`));
  const sources = [...headers, ...cells];
  if (sources.length === 0) return minPx;

  const layer = document.createElement('div');
  layer.setAttribute('aria-hidden', 'true');
  layer.style.position = 'absolute';
  layer.style.visibility = 'hidden';
  layer.style.pointerEvents = 'none';
  layer.style.left = '0';
  layer.style.top = '0';
  layer.style.width = 'max-content';
  layer.style.whiteSpace = 'nowrap';
  container.appendChild(layer);

  try {
    const pairs = sources.map((src) => {
      const cs = getComputedStyle(src);
      const holder = document.createElement('div');
      holder.style.width = 'max-content';
      holder.style.whiteSpace = 'nowrap';
      holder.style.font = cs.font;
      holder.style.letterSpacing = cs.letterSpacing;
      holder.style.fontVariantNumeric = cs.fontVariantNumeric;
      const clone = src.cloneNode(true) as HTMLElement;
      for (const el of [clone, ...Array.from(clone.querySelectorAll<HTMLElement>('.truncate'))]) {
        el.classList.remove('truncate');
        el.style.width = 'max-content';
        el.style.maxWidth = 'none';
        el.style.overflow = 'visible';
        el.style.textOverflow = 'clip';
      }
      holder.appendChild(clone);
      layer.appendChild(holder);
      return holder;
    });

    let widest = 0;
    for (const holder of pairs) widest = Math.max(widest, holder.getBoundingClientRect().width);

    const th = container.querySelector(`th${sel}`);
    const td = container.querySelector(`td${sel}`);
    const extra = Math.max(th ? frame(th) : 0, td ? frame(td) : 0);
    return Math.min(maxPx, Math.max(minPx, Math.ceil(widest + extra)));
  } finally {
    layer.remove();
  }
}
