import type { LayoutSlotId } from './slots';
import { stableStringify } from './stable';

export type LayoutStatus = 'loading' | 'ready' | 'error';

export interface LayoutEntry {
  v: number;
  data: unknown;
}
export interface LayoutDoc {
  v: 1;
  slots: Record<string, LayoutEntry>;
}

export interface SlotSpec<T> {
  id: LayoutSlotId;
  version: number;
  defaults: () => T;
  /** 校验 data；不合法返回 null，改用默认值。 */
  parse: (data: unknown) => T | null;
}

export interface LayoutStore {
  status(): LayoutStatus;
  /** 同一份原始数据返回同一个引用。 */
  get<T>(spec: SlotSpec<T>): T;
  /** status 不是 ready 时忽略。 */
  set<T>(spec: SlotSpec<T>, value: T): void;
  /** status 不是 ready 时忽略。 */
  reset(id: LayoutSlotId): void;
  /** 任何变化都通知；槽位没变时 get 返回同一个引用，useSyncExternalStore 不会重新渲染。 */
  subscribe(cb: () => void): () => void;
  isDirty(): boolean;
  /** 加载成功：已保存 = 当前 = doc，status = ready。 */
  loaded(doc: LayoutDoc | null): void;
  /** 加载失败：status = error。 */
  failed(): void;
  /** 当前布局，含没访问过的槽位。返回的是拷贝。 */
  snapshot(): LayoutDoc;
  /** 已保存 = doc，当前不变。 */
  markSaved(doc: LayoutDoc): void;
  revertToSaved(): void;
  resetAll(): void;
}

type Slots = Record<string, LayoutEntry>;

interface ParsedCache {
  /** 解析时的原始数据（槽位不存在时为 undefined）。 */
  raw: LayoutEntry | undefined;
  version: number;
  value: unknown;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/** 只保留形状合法的槽位（对象，且 v 是数字），并做深拷贝。 */
function cloneSlots(src: unknown): Slots {
  const out: Slots = {};
  if (!isRecord(src)) return out;
  for (const [id, entry] of Object.entries(src)) {
    if (!isRecord(entry) || typeof entry.v !== 'number') continue;
    out[id] = { v: entry.v, data: cloneData(entry.data) };
  }
  return out;
}

function cloneData(data: unknown): unknown {
  if (data === undefined) return undefined;
  return JSON.parse(JSON.stringify(data));
}

function entryKey(e: LayoutEntry | undefined): string {
  return e === undefined ? '' : stableStringify(e);
}

export function createLayoutStore(): LayoutStore {
  let status: LayoutStatus = 'loading';
  let saved: Slots = {};
  let current: Slots = {};
  const listeners = new Set<() => void>();
  /** 按槽位缓存的解析结果。 */
  const parsed = new Map<string, ParsedCache>();
  /** 与已保存不同的槽位；只在会改动数据的操作后重算。 */
  const dirtyIds = new Set<string>();

  function notify() {
    for (const cb of [...listeners]) cb();
  }

  function recomputeDirty(id?: string) {
    if (id !== undefined) {
      if (entryKey(current[id]) !== entryKey(saved[id])) dirtyIds.add(id);
      else dirtyIds.delete(id);
      return;
    }
    dirtyIds.clear();
    const ids = new Set([...Object.keys(current), ...Object.keys(saved)]);
    for (const k of ids) {
      if (entryKey(current[k]) !== entryKey(saved[k])) dirtyIds.add(k);
    }
  }

  function get<T>(spec: SlotSpec<T>): T {
    const raw = current[spec.id];
    const hit = parsed.get(spec.id);
    if (hit && hit.raw === raw && hit.version === spec.version) return hit.value as T;
    let value: T | null = null;
    if (raw !== undefined && raw.v === spec.version) value = spec.parse(raw.data);
    if (value === null) value = spec.defaults();
    parsed.set(spec.id, { raw, version: spec.version, value });
    return value;
  }

  return {
    status: () => status,
    get,
    set(spec, value) {
      if (status !== 'ready') return;
      const prev = current[spec.id];
      if (prev && prev.v === spec.version && stableStringify(prev.data) === stableStringify(value)) return;
      current = { ...current, [spec.id]: { v: spec.version, data: value } };
      recomputeDirty(spec.id);
      notify();
    },
    reset(id) {
      if (status !== 'ready') return;
      if (!(id in current)) return;
      const next = { ...current };
      delete next[id];
      current = next;
      recomputeDirty(id);
      notify();
    },
    subscribe(cb) {
      listeners.add(cb);
      return () => {
        listeners.delete(cb);
      };
    },
    isDirty: () => dirtyIds.size > 0,
    loaded(doc) {
      let slots: unknown = doc?.slots;
      if (doc && (doc as { v: unknown }).v !== 1) {
        console.warn(`布局文档版本 ${String((doc as { v: unknown }).v)} 不受支持，使用默认布局`);
        slots = {};
      }
      saved = cloneSlots(slots);
      current = cloneSlots(slots);
      parsed.clear();
      status = 'ready';
      recomputeDirty();
      notify();
    },
    failed() {
      status = 'error';
      notify();
    },
    snapshot() {
      return { v: 1, slots: cloneSlots(current) };
    },
    markSaved(doc) {
      saved = cloneSlots(doc.slots);
      recomputeDirty();
      notify();
    },
    revertToSaved() {
      // 没改动的槽位沿用当前对象，保证它们的 get 引用不变
      const next: Slots = {};
      for (const [id, entry] of Object.entries(saved)) {
        const cur = current[id];
        next[id] = cur !== undefined && entryKey(cur) === entryKey(entry) ? cur : cloneSlots({ [id]: entry })[id];
      }
      current = next;
      recomputeDirty();
      notify();
    },
    resetAll() {
      current = {};
      recomputeDirty();
      notify();
    },
  };
}
