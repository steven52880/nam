/**
 * 稳定序列化：对象键按字典序排序，数组保持顺序。
 * 用于比较两份布局数据是否相同，不用于持久化。
 * 对象里值为 undefined（或函数、symbol）的键被跳过，与 JSON.stringify 一致；
 * 数组里的这类元素写成 null。
 */
export function stableStringify(value: unknown): string {
  return stringify(value) ?? 'null';
}

function stringify(value: unknown): string | undefined {
  if (value === null) return 'null';
  switch (typeof value) {
    case 'string':
      return JSON.stringify(value);
    case 'number':
      return Number.isFinite(value) ? JSON.stringify(value) : 'null';
    case 'boolean':
      return value ? 'true' : 'false';
    case 'bigint':
      return value.toString();
    case 'undefined':
    case 'function':
    case 'symbol':
      return undefined;
    default:
      break;
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stringify(item) ?? 'null').join(',')}]`;
  }
  const obj = value as Record<string, unknown>;
  if (typeof (obj as { toJSON?: unknown }).toJSON === 'function') {
    return stringify((obj as { toJSON: () => unknown }).toJSON());
  }
  const parts: string[] = [];
  for (const key of Object.keys(obj).sort()) {
    const s = stringify(obj[key]);
    if (s !== undefined) parts.push(`${JSON.stringify(key)}:${s}`);
  }
  return `{${parts.join(',')}}`;
}
