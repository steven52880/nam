import { createContext, useContext } from 'react';
import type { Session } from '@/api/types';

export const SessionContext = createContext<Session | null>(null);

export function useSession(): Session {
  const s = useContext(SessionContext);
  if (!s) throw new Error('SessionContext 缺失');
  return s;
}
