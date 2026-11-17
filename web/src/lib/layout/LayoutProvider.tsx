import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { toast } from 'sonner';
import { createLayoutStore, type LayoutDoc, type LayoutStatus, type LayoutStore, type SlotSpec } from './store';

export interface LayoutProviderProps {
  /** 加载已保存的布局；不传则直接使用空布局（全部默认，立即可编辑，只在内存里）。 */
  load?: () => Promise<LayoutDoc | null>;
  /** 保存布局；不传则保存按钮是空操作。 */
  save?: (doc: LayoutDoc) => Promise<unknown>;
  children: ReactNode;
}

interface LayoutContextValue {
  store: LayoutStore;
  saving: boolean;
  save: () => void;
  retry: () => void;
}

const LayoutContext = createContext<LayoutContextValue | null>(null);

/** 没有 Provider 时（例如单独渲染一个表格）使用的内存仓库：全部默认，立即可编辑。 */
let fallback: LayoutContextValue | null = null;
function fallbackValue(): LayoutContextValue {
  if (!fallback) {
    const store = createLayoutStore();
    store.loaded(null);
    fallback = { store, saving: false, save: () => {}, retry: () => {} };
  }
  return fallback;
}

function useLayoutContext(): LayoutContextValue {
  return useContext(LayoutContext) ?? fallbackValue();
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/**
 * 布局仓库的 React 接入。换用户时由使用方加 key 重建。
 * - 加载：挂载时调用 load()；每次加载带递增的代次，卸载或重试后旧代次的响应直接丢弃。
 * - 加载完成前 status = loading，加载失败 status = error，两者都不允许调整布局。
 * - 保存：点击时取 snapshot() 固定下来；不允许并发保存；成功后只 markSaved(那份快照)，
 *   所以保存期间的新调整仍然显示为"未保存"；失败时提示错误，状态不变。
 */
export function LayoutProvider({ load, save, children }: LayoutProviderProps) {
  const [store] = useState(() => {
    const s = createLayoutStore();
    if (!load) s.loaded(null);
    return s;
  });
  const [saving, setSaving] = useState(false);
  const generation = useRef(0);
  const savingRef = useRef(false);
  const mounted = useRef(true);
  const loadRef = useRef(load);
  loadRef.current = load;
  const saveRef = useRef(save);
  saveRef.current = save;

  const startLoad = useCallback(() => {
    const fn = loadRef.current;
    if (!fn) return;
    const gen = ++generation.current;
    Promise.resolve()
      .then(() => fn())
      .then(
        (doc) => {
          if (gen === generation.current) store.loaded(doc);
        },
        () => {
          if (gen === generation.current) store.failed();
        },
      );
  }, [store]);

  useEffect(() => {
    mounted.current = true;
    startLoad();
    return () => {
      mounted.current = false;
      generation.current += 1;
    };
  }, [startLoad]);

  const retry = useCallback(() => {
    if (store.status() === 'ready') return;
    startLoad();
  }, [store, startLoad]);

  const doSave = useCallback(() => {
    const fn = saveRef.current;
    if (!fn || savingRef.current || store.status() !== 'ready' || !store.isDirty()) return;
    const snapshot = store.snapshot();
    savingRef.current = true;
    setSaving(true);
    Promise.resolve()
      .then(() => fn(snapshot))
      .then(
        () => {
          store.markSaved(snapshot);
        },
        (err: unknown) => {
          toast.error(`保存布局失败：${errorMessage(err)}`);
        },
      )
      .finally(() => {
        savingRef.current = false;
        if (mounted.current) setSaving(false);
      });
  }, [store]);

  const value = useMemo<LayoutContextValue>(
    () => ({ store, saving, save: doSave, retry }),
    [store, saving, doSave, retry],
  );
  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}

/**
 * 读写一个槽位：[value, set, reset]。spec 必须是稳定的对象（模块级常量或按 id 缓存），
 * 其它槽位变化时 value 的引用不变，不会重新渲染。
 */
export function useLayoutSlot<T>(spec: SlotSpec<T>): [T, (value: T) => void, () => void] {
  const { store } = useLayoutContext();
  const value = useSyncExternalStore(store.subscribe, () => store.get(spec));
  const set = useCallback((v: T) => store.set(spec, v), [store, spec]);
  const reset = useCallback(() => store.reset(spec.id), [store, spec]);
  return [value, set, reset];
}

/** 布局是否已加载完成、允许调整（表格和分栏用它决定是否允许拖动、改宽等）。 */
export function useLayoutEditable(): boolean {
  const { store } = useLayoutContext();
  return useSyncExternalStore(store.subscribe, () => store.status() === 'ready');
}

export interface LayoutControls {
  status: LayoutStatus;
  dirty: boolean;
  saving: boolean;
  save: () => void;
  retry: () => void;
  revertToSaved: () => void;
  resetAll: () => void;
}

/** 给顶栏按钮用。 */
export function useLayoutControls(): LayoutControls {
  const { store, saving, save: doSave, retry } = useLayoutContext();
  const status = useSyncExternalStore(store.subscribe, store.status);
  const dirty = useSyncExternalStore(store.subscribe, store.isDirty);
  return useMemo(
    () => ({
      status,
      dirty,
      saving,
      save: doSave,
      retry,
      revertToSaved: store.revertToSaved,
      resetAll: store.resetAll,
    }),
    [status, dirty, saving, doSave, retry, store],
  );
}
