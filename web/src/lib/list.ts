/** 把逗号 / 换行 / 制表符分隔的文本拆成去重、去空白的列表。 */
export function splitList(text: string): string[] {
  return uniq(
    text
      .split(/[,，\n\r\t]+/)
      .map((s) => s.trim())
      .filter(Boolean),
  );
}

export function uniq(items: string[]): string[] {
  return [...new Set(items)];
}

/** 安全解析 JSON 字符串数组；失败返回空数组。 */
export function parseStringArray(json: string): string[] {
  try {
    const parsed: unknown = JSON.parse(json);
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : [];
  } catch {
    return [];
  }
}
