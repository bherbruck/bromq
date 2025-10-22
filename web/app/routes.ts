import { type RouteConfig, index, layout, route } from '@react-router/dev/routes'

export default [
  index('routes/home.tsx'),
  route('login', 'routes/login.tsx'),
  layout('components/app-layout.tsx', [
    route('dashboard', 'routes/dashboard.tsx'),
    route('clients', 'routes/clients.tsx'),
    route('clients/:id', 'routes/clients.$id.tsx'),
    route('mqtt-users', 'routes/mqtt-users.tsx'),
    route('mqtt-users/:id', 'routes/mqtt-users.$id.tsx'),
    route('users', 'routes/users.tsx'),
    route('acl', 'routes/acl.tsx'),
  ]),
] satisfies RouteConfig
