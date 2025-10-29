import { Activity, ArrowDown, ArrowUp, Server, Users } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Route } from './+types/dashboard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { api, type Metrics } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'Dashboard - BroMQ' }]

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

function formatDuration(nanoseconds: number): string {
  // Convert nanoseconds to seconds
  const seconds = Math.floor(nanoseconds / 1000000000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) return `${days}d ${hours % 24}h`
  if (hours > 0) return `${hours}h ${minutes % 60}m`
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`
  return `${seconds}s`
}

export default function DashboardPage() {
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const data = await api.getMetrics()
        setMetrics(data)
      } catch (error) {
        console.error('Failed to fetch metrics:', error)
      } finally {
        setIsLoading(false)
      }
    }

    fetchMetrics()
    const interval = setInterval(fetchMetrics, 5000) // Refresh every 5 seconds

    return () => clearInterval(interval)
  }, [])

  if (isLoading) {
    return <div className="text-muted-foreground">Loading metrics...</div>
  }

  if (!metrics) {
    return <div className="text-destructive">Failed to load metrics</div>
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Connected Clients</CardTitle>
            <Users className="text-muted-foreground h-4 w-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.connected_clients}</div>
            <p className="text-muted-foreground text-xs">
              {metrics.total_clients} total connections
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Server Uptime</CardTitle>
            <Server className="text-muted-foreground h-4 w-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatDuration(metrics.uptime)}</div>
            <p className="text-muted-foreground text-xs">Running continuously</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Messages Received</CardTitle>
            <ArrowDown className="text-muted-foreground h-4 w-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.messages_received.toLocaleString()}</div>
            <p className="text-muted-foreground text-xs">
              {metrics.packets_received.toLocaleString()} packets
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Messages Sent</CardTitle>
            <ArrowUp className="text-muted-foreground h-4 w-4" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{metrics.messages_sent.toLocaleString()}</div>
            <p className="text-muted-foreground text-xs">
              {metrics.packets_sent.toLocaleString()} packets
            </p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Traffic Statistics</CardTitle>
            <CardDescription>Network bandwidth usage</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Bytes Received</span>
              <span className="text-muted-foreground text-sm">
                {formatBytes(metrics.bytes_received)}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Bytes Sent</span>
              <span className="text-muted-foreground text-sm">
                {formatBytes(metrics.bytes_sent)}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Messages Dropped</span>
              <span className="text-muted-foreground text-sm">
                {metrics.messages_dropped.toLocaleString()}
              </span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Broker Statistics</CardTitle>
            <CardDescription>Server state information</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Active Subscriptions</span>
              <span className="text-muted-foreground text-sm">
                {metrics.subscriptions_total.toLocaleString()}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Retained Messages</span>
              <span className="text-muted-foreground text-sm">
                {metrics.retained_messages.toLocaleString()}
              </span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">Connected Clients</span>
              <span className="text-muted-foreground text-sm">{metrics.connected_clients}</span>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
