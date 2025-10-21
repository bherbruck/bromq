import { Outlet, useLocation } from 'react-router'
import { SidebarProvider, SidebarTrigger, SidebarInset } from '~/components/ui/sidebar'
import { AppSidebar } from './app-sidebar'
import { ProtectedRoute } from './protected-route'

const pageTitles: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/clients': 'Connected Clients',
  '/users': 'Users',
  '/acl': 'ACL Rules',
}

export default function AppLayout() {
  const location = useLocation()
  const pageTitle = pageTitles[location.pathname] || 'MQTT Server'

  return (
    <ProtectedRoute>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <header className="sticky top-0 z-10 flex h-14 items-center gap-4 border-b bg-background px-6">
            <SidebarTrigger />
            <h1 className="text-base font-semibold">{pageTitle}</h1>
          </header>
          <main className="flex-1 p-4 lg:p-6">
            <div className="container mx-auto max-w-7xl">
              <Outlet />
            </div>
          </main>
        </SidebarInset>
      </SidebarProvider>
    </ProtectedRoute>
  )
}
