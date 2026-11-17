// 架构检查：像素值、分层、查询/提交位置、禁用写法。只用 Node 自带模块。
// 用法：node scripts/check.mjs [--all]   （--all：忽略 enabled，运行全部规则，用于预览后续批次）
// 例外：行尾 `// check-ignore: 规则名 原因`（css 里 `/* check-ignore: 规则名 原因 */`）。
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, posix, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const SRC = join(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const RUN_ALL = process.argv.includes('--all');

// ───────── 文件收集 ─────────
function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.(ts|tsx|css)$/.test(name) && !/\.test\.ts$/.test(name)) out.push(full);
  }
  return out;
}

// ───────── import 解析与路径规范化 ─────────
/** 把 import 路径换算成相对 src/ 的路径（去掉扩展名）；外部包返回 null。 */
function normalizeImport(spec, fromRel) {
  let p;
  if (spec.startsWith('@/')) p = spec.slice(2);
  else if (spec.startsWith('./') || spec.startsWith('../')) p = posix.normalize(posix.join(posix.dirname(fromRel), spec));
  else return null;
  return p.replace(/\.(tsx?|css)$/, '').replace(/\/index$/, '');
}

const IMPORT_RE = /(^|[\n;])[ \t]*(import|export)\s+(type\s+)?([^;'"`]*?)\s*from\s*(['"])([^'"\n]+)\5|(^|[\n;])[ \t]*import\s*(['"])([^'"\n]+)\8|\bimport\(\s*(['"])([^'"\n]+)\10\s*\)/g;

/** 返回 [{ line, spec, path, typeOnly }]；line 是 import 语句首行（含关键字）的行号。 */
function parseImports(text, rel) {
  const result = [];
  for (const m of text.matchAll(IMPORT_RE)) {
    const spec = m[6] ?? m[9] ?? m[11];
    let typeOnly = false;
    if (m[2]) {
      const clause = (m[4] ?? '').trim();
      typeOnly = !!m[3];
      if (!typeOnly && /^\{[^}]*\}$/.test(clause)) {
        const names = clause.slice(1, -1).split(',').map((s) => s.trim()).filter(Boolean);
        typeOnly = names.length > 0 && names.every((n) => /^type\s/.test(n));
      }
    }
    const kwOffset = m.index + (m[0].length - m[0].trimStart().length);
    const line = text.slice(0, kwOffset).split('\n').length;
    result.push({ line, spec, path: normalizeImport(spec, rel), typeOnly });
  }
  return result;
}

// ───────── 分层 ─────────
const L5_DIRS = ['table', 'cells', 'form', 'wizard', 'filters', 'dnd', 'data-grid'];
const L5_FILES = ['ConfirmDialog', 'ChipEditor', 'PriceText'];
const L6_DIRS = ['pickers', 'entities', 'model-picker'];
const L7_DIRS = ['layout'];

/** 相对 src/ 的路径（无扩展名）→ 层级 1–7；不属于任何层返回 null。 */
function layerOf(p) {
  const seg = p.split('/');
  if (seg[0] === 'lib') return 1;
  if (seg[0] === 'api') return 2;
  if (seg[0] === 'hooks') return 3;
  if (seg[0] === 'pages' || seg[0] === 'app') return 7;
  if (seg[0] === 'components') {
    if (seg[1] === 'ui') return 4;
    if (L5_DIRS.includes(seg[1]) || L5_FILES.includes(seg[1])) return 5;
    if (L6_DIRS.includes(seg[1])) return 6;
    if (L7_DIRS.includes(seg[1])) return 7;
  }
  return null;
}

/** pages/Channels.tsx 与 pages/channels/** 同属一个页面。 */
function pageOf(p) {
  const seg = p.split('/');
  return seg[0] === 'pages' && seg[1] ? seg[1].toLowerCase() : null;
}

const isApiTypes = (p) => p === 'api/types' || p.startsWith('api/types');

// ───────── 规则 ─────────
// 每条规则：name、batch（从第几批开始生效）、enabled、desc、run(file) → [{ line, msg }]
// file = { rel（相对 src/，含扩展名）, relNoExt, text, lines, ext, imports() }
function lineRule(re, msg, allowFiles) {
  return (file) => {
    if (allowFiles?.includes(file.relNoExt)) return [];
    const out = [];
    file.lines.forEach((l, i) => {
      if (re.test(l)) out.push({ line: i + 1, msg });
    });
    return out;
  };
}

const RULES = [
  // ── 第一批 ──
  {
    name: '像素值',
    batch: 1,
    enabled: true,
    desc: '不要写 [数字px] / 数字px（1px 除外）；样式用 rem，代码里用 remToPx()',
    run: (file) => {
      const out = [];
      file.lines.forEach((l, i) => {
        for (const m of l.matchAll(/(?<![\w.])(\d+(?:\.\d+)?)px\b/g)) {
          if (m[1] === '1') continue;
          out.push({ line: i + 1, msg: `${m[0]}：改用 rem 或 lib/size.ts` });
        }
      });
      return out;
    },
  },
  {
    name: '分层',
    batch: 1,
    enabled: true,
    desc: '只能引用同层或更低层；页面之间不能互相引用；L1–L5 对 api/ 只能 import type 且只来自 api/types*',
    run: (file) => {
      if (file.ext === '.css') return [];
      const from = file.relNoExt;
      const myLayer = layerOf(from);
      if (myLayer === null) return [];
      const myPage = pageOf(from);
      const out = [];
      for (const imp of file.imports()) {
        if (imp.path === null) continue;
        const target = layerOf(imp.path);
        if (target === null) continue;
        if (target === 2 && (myLayer === 1 || myLayer === 4 || myLayer === 5)) {
          // L1–L5 对 api/ 只能 import type，且只来自 api/types*
          if (!(imp.typeOnly && isApiTypes(imp.path))) {
            out.push({ line: imp.line, msg: `L${myLayer} 对 api/ 只能 import type 且来自 api/types*：${imp.path}` });
          }
          continue;
        }
        if (target > myLayer) {
          out.push({ line: imp.line, msg: `L${myLayer} 不能引用 L${target}：${imp.path}` });
          continue;
        }
        const targetPage = pageOf(imp.path);
        if (myPage && targetPage && myPage !== targetPage) {
          out.push({ line: imp.line, msg: `页面之间不能互相引用：${imp.path}` });
        }
      }
      return out;
    },
  },
  {
    name: '查询位置',
    batch: 1,
    enabled: true,
    desc: 'useQuery( / useQueries( 只能出现在 api/queries.ts',
    run: lineRule(/\buseQuery\s*\(|\buseQueries\s*\(/, 'useQuery/useQueries 只能写在 api/queries.ts', ['api/queries']),
  },
  {
    name: '查询键位置',
    batch: 1,
    enabled: true,
    desc: '手写查询键数组 queryKey: [ 只能出现在 api/keys.ts（queryKey: keys.xxx() 合法）',
    run: lineRule(/\bqueryKey\s*:\s*\[/, '手写查询键只能写在 api/keys.ts，其它处用 keys.xxx()', ['api/keys']),
  },
  {
    name: '提交位置',
    batch: 1,
    enabled: true,
    desc: 'useMutation( / toast.error(errorText 只能出现在 hooks/useActionMutation.ts',
    run: lineRule(/\buseMutation\s*\(|toast\.error\(\s*errorText/, '提交只能写在 hooks/useActionMutation.ts', ['hooks/useActionMutation']),
  },
  // ── 第二批 ──
  {
    name: '重复组件',
    batch: 2,
    enabled: true,
    desc: 'function Field / Line / Section 只能出现在 form/Field.tsx、ui/section.tsx',
    run: lineRule(/\bfunction\s+(Field|Line|Section)\b/, 'Field/Line/Section 只能定义在 components/form/Field.tsx、components/ui/section.tsx', [
      'components/form/Field',
      'components/ui/section',
    ]),
  },
  {
    name: '提示条样式',
    batch: 2,
    enabled: true,
    desc: 'bg-destructive/10、bg-warning/15 只能出现在 ui/banner.tsx',
    run: lineRule(/bg-destructive\/10|bg-warning\/15/, '提示条样式只能写在 components/ui/banner.tsx', ['components/ui/banner', 'components/ui/misc']),
  },
  {
    name: '弹窗宽度',
    batch: 2,
    enabled: true,
    desc: 'pages/、components/entities/ 下的 DialogContent 不能带 max-w-（用 size）',
    run: (file) => {
      if (!(file.relNoExt.startsWith('pages/') || file.relNoExt.startsWith('components/entities/'))) return [];
      const out = [];
      for (const m of file.text.matchAll(/<DialogContent\b/g)) {
        // 取到标签结束（第一个不属于 => 的 >）
        const rest = file.text.slice(m.index);
        const end = rest.search(/(?<!=)>/);
        const tag = end === -1 ? rest : rest.slice(0, end);
        if (/max-w-/.test(tag)) {
          out.push({ line: file.text.slice(0, m.index).split('\n').length, msg: 'DialogContent 不能带 max-w-，改用 size 属性' });
        }
      }
      return out;
    },
  },
  {
    name: '网址参数',
    batch: 2,
    enabled: true,
    desc: 'useSearchParams( 只能出现在 lib/url-state.ts',
    run: lineRule(/\buseSearchParams\s*\(/, 'useSearchParams 只能写在 lib/url-state.ts', ['lib/url-state']),
  },
  // ── 第三批 ──
  {
    name: '旧表格',
    batch: 3,
    enabled: false,
    desc: '不能引用 components/data-grid、RowContextMenu',
    run: (file) => {
      if (file.ext === '.css' || file.relNoExt.startsWith('components/data-grid/')) return [];
      return file
        .imports()
        .filter((i) => i.path && (i.path === 'components/data-grid' || i.path.startsWith('components/data-grid/') || /(^|\/)RowContextMenu$/.test(i.path)))
        .map((i) => ({ line: i.line, msg: `不能引用旧表格：${i.path}` }));
    },
  },
  // ── 第四批 ──
  {
    name: '裸表格',
    batch: 4,
    enabled: false,
    desc: '<table 只能出现在 components/table/',
    run: (file) => (file.relNoExt.startsWith('components/table/') ? [] : lineRule(/<table\b/, '<table 只能写在 components/table/')(file)),
  },
  {
    name: '数据列表',
    batch: 4,
    enabled: false,
    desc: '不能有 <datalist（改用 Combobox）',
    run: lineRule(/<datalist\b/, '不能用 <datalist，改用 Combobox'),
  },
  {
    name: '下拉选择',
    batch: 4,
    enabled: false,
    desc: '@/components/ui/select 只能在白名单文件里使用',
    run: (file) => {
      if (file.ext === '.css') return [];
      const allowed = ['ChannelToolbar.tsx', 'UpstreamForm.tsx', 'VendorSection.tsx', 'Stats.tsx'];
      if (allowed.includes(posix.basename(file.rel)) || file.relNoExt === 'components/ui/select') return [];
      return file
        .imports()
        .filter((i) => i.path === 'components/ui/select')
        .map((i) => ({ line: i.line, msg: 'ui/select 只能在白名单文件使用（固定选项），其它改用 Combobox' }));
    },
  },
];

// ───────── 运行 ─────────
const files = walk(SRC).map((full) => {
  const rel = relative(SRC, full).split(sep).join('/');
  const text = readFileSync(full, 'utf8');
  let cached = null;
  return {
    rel,
    relNoExt: rel.replace(/\.(tsx?|css)$/, ''),
    ext: rel.slice(rel.lastIndexOf('.')),
    text,
    lines: text.split(/\r?\n/),
    imports() {
      cached ??= parseImports(text, rel);
      return cached;
    },
  };
});

const active = RULES.filter((r) => RUN_ALL || r.enabled);
const violations = [];
const ignored = [];

for (const file of files) {
  for (const rule of active) {
    for (const v of rule.run(file)) {
      const lineText = file.lines[v.line - 1] ?? '';
      const ig = [...lineText.matchAll(/check-ignore:\s*([^\s*]+)\s*([^*]*?)\s*(?:\*\/)?\s*$/g)].find((m) => m[1] === rule.name);
      if (ig) {
        ignored.push(`${file.rel}:${v.line} ${rule.name} ${ig[2]}`.trimEnd());
        continue;
      }
      violations.push(`${file.rel}:${v.line} ${rule.name} ${v.msg}`);
    }
  }
}

for (const v of violations) console.error(v);
if (ignored.length) {
  console.log(`check-ignore 例外 ${ignored.length} 处：`);
  for (const i of ignored) console.log(`  ${i}`);
}
console.log(
  `check：${files.length} 个文件，规则 ${active.map((r) => r.name).join('、')}，违规 ${violations.length} 处`,
);
process.exit(violations.length > 0 ? 1 : 0);
