import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { Toaster } from 'sonner';
import { setAuthHandlers } from '@/api/client';
import { keys } from '@/api/keys';
import { TooltipProvider } from '@/components/ui/misc';
import { routes } from '@/app/routes';
import './index.css';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, staleTime: 10_000 } },
});

const router = createBrowserRouter(routes, { basename: '/nam' });

setAuthHandlers({
  unauthorized: () => {
    queryClient.removeQueries({ queryKey: keys.session() });
    void router.navigate('/login', { replace: true });
  },
  forbidden: () => {
    void router.navigate('/forbidden', { replace: true });
  },
});

const root = document.getElementById('root');
if (!root) throw new Error('缺少 #root 节点');

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <RouterProvider router={router} />
        <Toaster richColors position="top-right" />
      </TooltipProvider>
    </QueryClientProvider>
  </StrictMode>,
);
