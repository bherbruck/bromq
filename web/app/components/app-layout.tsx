import { Fragment } from 'react'
import { Navigate, Outlet, useLocation, Link } from 'react-router'
import { SidebarProvider, SidebarTrigger, SidebarInset } from '~/components/ui/sidebar'
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '~/components/ui/breadcrumb'
import { AppSidebar } from './app-sidebar'

const routeNames: Record<string, string> = {
  'dashboard': 'Dashboard',
  'clients': 'Clients',
  'mqtt-users': 'MQTT Users',
  'users': 'Dashboard Users',
  'acl': 'ACL Rules',
  'bridges': 'Bridges',
  'scripts': 'Scripts',
}

export default function AppLayout() {
  const location = useLocation()

  // Check authentication
  const token = localStorage.getItem('mqtt_token')

  if (!token) {
    return <Navigate to="/login" replace />
  }

  // Build breadcrumbs from current path
  const pathSegments = location.pathname.split('/').filter(Boolean)
  const breadcrumbs = pathSegments.map((segment, index) => {
    const path = '/' + pathSegments.slice(0, index + 1).join('/')
    const isLast = index === pathSegments.length - 1
    const label = routeNames[segment] || segment

    return { path, label, isLast }
  })

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="sticky top-0 z-10 flex h-14 items-center gap-4 border-b bg-background px-6">
          <SidebarTrigger />
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink asChild>
                  <Link to="/dashboard">
                    Dashboard
                  </Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              {breadcrumbs.length > 0 && pathSegments[0] !== 'dashboard' && <BreadcrumbSeparator />}
              {breadcrumbs.map((crumb) => (
                <Fragment key={crumb.path}>
                  {crumb.path !== '/dashboard' && (
                    <>
                      <BreadcrumbItem>
                        {crumb.isLast ? (
                          <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                        ) : (
                          <BreadcrumbLink asChild>
                            <Link to={crumb.path}>{crumb.label}</Link>
                          </BreadcrumbLink>
                        )}
                      </BreadcrumbItem>
                      {!crumb.isLast && <BreadcrumbSeparator />}
                    </>
                  )}
                </Fragment>
              ))}
            </BreadcrumbList>
          </Breadcrumb>
        </header>
        <main className="flex-1 p-4 lg:p-6">
          <Outlet />
        </main>
      </SidebarInset>
    </SidebarProvider>
  )
}
