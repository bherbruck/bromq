import { Navigate } from 'react-router'
import type { Route } from './+types/home'

export const meta: Route.MetaFunction = () => [{ title: 'BroMQ' }]

export default function Home() {
  // Check if user is authenticated
  const token = localStorage.getItem('mqtt_token')

  // Redirect to dashboard if authenticated, otherwise to login
  return <Navigate to={token ? '/dashboard' : '/login'} replace />
}
