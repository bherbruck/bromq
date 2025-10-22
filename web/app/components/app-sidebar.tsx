import { Activity, Home, Key, Lock, LogOut, Server, Users } from 'lucide-react'
import { NavLink } from 'react-router'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '~/components/ui/sidebar'
import { useAuth } from '~/lib/auth-context'

const navigation = [
  {
    title: 'Overview',
    items: [
      { title: 'Dashboard', icon: Home, url: '/dashboard' },
      { title: 'Clients', icon: Activity, url: '/clients' },
    ],
  },
  {
    title: 'Management',
    items: [
      { title: 'MQTT Users', icon: Key, url: '/mqtt-users' },
      { title: 'ACL Rules', icon: Lock, url: '/acl' },
    ],
  },
  {
    title: 'System',
    items: [
      { title: 'Dashboard Users', icon: Users, url: '/users' },
    ],
  },
]

export function AppSidebar() {
  const { user, logout } = useAuth()

  return (
    <Sidebar>
      <SidebarHeader className="flex h-14 flex-row items-center gap-2 border-b px-6">
        <Server className="h-5 w-5" />
        <h2 className="text-base font-semibold">MQTT Server</h2>
      </SidebarHeader>

      <SidebarContent>
        {navigation.map((group) => (
          <SidebarGroup key={group.title}>
            <SidebarGroupLabel>{group.title}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.title}>
                    <SidebarMenuButton asChild>
                      <NavLink
                        to={item.url}
                        className={({ isActive }) =>
                          isActive ? 'bg-accent text-accent-foreground' : ''
                        }
                      >
                        <item.icon className="h-4 w-4" />
                        <span>{item.title}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="border-t p-4">
        <div className="mb-2 px-2">
          <p className="text-muted-foreground text-xs">Logged in as</p>
          <p className="text-sm font-medium">{user?.username}</p>
        </div>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton onClick={logout}>
              <LogOut className="h-4 w-4" />
              <span>Logout</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}
