import { ChevronRight, Home, Inbox } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import type { Route } from './+types/clients.$id'
import { Badge } from '~/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '~/components/ui/chart'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { Spinner } from '~/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'
import { api, type ClientDetails, type ClientMetrics } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'Client Details - MQTT Server' }]

type MetricsHistoryPoint = ClientMetrics & { timestamp: number }

type RateDataPoint = {
  timestamp: number
  messages_received_rate: number
  messages_sent_rate: number
  bytes_received_rate: number
  bytes_sent_rate: number
}

export default function ClientDetailPage() {
  const { id } = useParams()
  const [client, setClient] = useState<ClientDetails | null>(null)
  const [metrics, setMetrics] = useState<ClientMetrics | null>(null)
  const [metricsHistory, setMetricsHistory] = useState<MetricsHistoryPoint[]>([])
  const [rateData, setRateData] = useState<RateDataPoint[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchClientData = async () => {
      if (!id) return

      try {
        const [clientData, metricsData] = await Promise.all([
          api.getClientDetails(id),
          api.getClientMetrics(id),
        ])
        setClient(clientData)
        setMetrics(metricsData)

        const now = Date.now()
        const newPoint = { ...metricsData, timestamp: now }

        // Add to history (keep last 100 points = 5 minutes at 3-second intervals)
        setMetricsHistory((prev) => {
          const updated = [...prev, newPoint]
          return updated.slice(-100)
        })

        // Calculate rates per second
        setMetricsHistory((prev) => {
          if (prev.length < 2) return prev

          const lastPoint = prev[prev.length - 2]
          const timeDiffSec = (now - lastPoint.timestamp) / 1000

          if (timeDiffSec > 0) {
            // Handle counter resets like Prometheus:
            // If new value < old value, assume counter reset and use new value as delta
            const calcDelta = (newVal: number, oldVal: number) =>
              newVal >= oldVal ? newVal - oldVal : newVal

            const messagesReceivedDelta = calcDelta(
              metricsData.messages_received,
              lastPoint.messages_received,
            )
            const messagesSentDelta = calcDelta(metricsData.messages_sent, lastPoint.messages_sent)
            const bytesReceivedDelta = calcDelta(
              metricsData.bytes_received,
              lastPoint.bytes_received,
            )
            const bytesSentDelta = calcDelta(metricsData.bytes_sent, lastPoint.bytes_sent)

            // Only add rate data if there's any activity (non-zero delta)
            const hasActivity =
              messagesReceivedDelta > 0 ||
              messagesSentDelta > 0 ||
              bytesReceivedDelta > 0 ||
              bytesSentDelta > 0

            if (hasActivity) {
              const rate: RateDataPoint = {
                timestamp: now,
                messages_received_rate: messagesReceivedDelta / timeDiffSec,
                messages_sent_rate: messagesSentDelta / timeDiffSec,
                bytes_received_rate: bytesReceivedDelta / timeDiffSec,
                bytes_sent_rate: bytesSentDelta / timeDiffSec,
              }

              setRateData((prevRates) => {
                const updated = [...prevRates, rate]
                return updated.slice(-100)
              })
            }
          }

          return prev
        })
      } catch (error) {
        console.error('Failed to fetch client data:', error)
        setError('Failed to load client details')
      } finally {
        setIsLoading(false)
      }
    }

    fetchClientData()
    const interval = setInterval(fetchClientData, 3000) // Refresh every 3 seconds

    return () => clearInterval(interval)
  }, [id])

  const getProtocolVersion = (version: number) => {
    const versions: Record<number, string> = {
      3: 'MQTT 3.1',
      4: 'MQTT 3.1.1',
      5: 'MQTT 5',
    }
    return versions[version] || `v${version}`
  }

  if (isLoading) {
    return (
      <div className="text-muted-foreground flex items-center gap-2">
        <Spinner />
        Loading client details...
      </div>
    )
  }

  if (error || !client) {
    return (
      <div className="space-y-4">
        <nav className="text-muted-foreground flex items-center gap-2 text-sm">
          <Link to="/dashboard" className="hover:text-foreground">
            <Home className="h-4 w-4" />
          </Link>
          <ChevronRight className="h-4 w-4" />
          <Link to="/clients" className="hover:text-foreground">
            Clients
          </Link>
        </nav>
        <Card>
          <CardContent className="py-8">
            <div className="text-center">
              <p className="text-destructive">{error || 'Client not found'}</p>
              <Link to="/clients" className="text-primary mt-4 inline-block">
                Back to Clients
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumbs */}
      <nav className="text-muted-foreground flex items-center gap-2 text-sm">
        <Link to="/dashboard" className="hover:text-foreground">
          <Home className="h-4 w-4" />
        </Link>
        <ChevronRight className="h-4 w-4" />
        <Link to="/clients" className="hover:text-foreground">
          Clients
        </Link>
        <ChevronRight className="h-4 w-4" />
        <span className="text-foreground font-medium">{client.id}</span>
      </nav>

      {/* Client Overview */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Client: <span className="font-mono text-base">{client.id}</span>
          </CardTitle>
          <CardDescription>
            {client.username || 'Anonymous'} • {getProtocolVersion(client.protocol_version)}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-6 md:grid-cols-4">
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Username</p>
              <p className="font-medium">{client.username || 'anonymous'}</p>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Remote Address</p>
              <p className="font-mono text-sm">{client.remote}</p>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Listener</p>
              <Badge variant="outline">{client.listener}</Badge>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Protocol Version</p>
              <Badge variant="outline">{getProtocolVersion(client.protocol_version)}</Badge>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Keepalive</p>
              <p>{client.keepalive}s</p>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Clean Session</p>
              <Badge variant={client.clean ? 'default' : 'secondary'}>
                {client.clean ? 'Yes' : 'No'}
              </Badge>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">Subscriptions</p>
              <p className="text-lg font-semibold">{client.subscriptions.length}</p>
            </div>
            <div>
              <p className="text-muted-foreground mb-1 text-sm">In-Flight Messages</p>
              <p className="text-lg font-semibold">{client.inflight_count}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Subscriptions */}
      <Card>
        <CardHeader>
          <CardTitle>Subscriptions</CardTitle>
          <CardDescription>
            {client.subscriptions.length} active subscription
            {client.subscriptions.length !== 1 ? 's' : ''}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {client.subscriptions.length === 0 ? (
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Inbox />
                </EmptyMedia>
                <EmptyTitle>No subscriptions</EmptyTitle>
                <EmptyDescription>This client is not subscribed to any topics</EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Topic Pattern</TableHead>
                  <TableHead>QoS Level</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {client.subscriptions.map((sub, idx) => (
                  <TableRow key={idx}>
                    <TableCell className="font-mono">{sub.topic}</TableCell>
                    <TableCell>
                      <Badge variant="outline">QoS {sub.qos}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Metrics Overview */}
      {metrics && (
        <Card>
          <CardHeader>
            <CardTitle>Client Metrics</CardTitle>
            <CardDescription>
              Real-time statistics - in-memory only, resets when client disconnects
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Messages Received</p>
                <p className="text-2xl font-bold">{metrics.messages_received.toLocaleString()}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Messages Sent</p>
                <p className="text-2xl font-bold">{metrics.messages_sent.toLocaleString()}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Bytes Received</p>
                <p className="text-2xl font-bold">{formatBytes(metrics.bytes_received)}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Bytes Sent</p>
                <p className="text-2xl font-bold">{formatBytes(metrics.bytes_sent)}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Packets Received</p>
                <p className="text-2xl font-bold">{metrics.packets_received.toLocaleString()}</p>
              </div>
              <div className="space-y-1">
                <p className="text-muted-foreground text-sm">Packets Sent</p>
                <p className="text-2xl font-bold">{metrics.packets_sent.toLocaleString()}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Charts */}
      <Card>
        <CardHeader>
          <CardTitle>Message Rate</CardTitle>
          <CardDescription>PUBLISH messages per second</CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer config={messageChartConfig} className="h-[400px] w-full">
            <LineChart data={rateData}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="timestamp"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                tickFormatter={(value) =>
                  new Date(value).toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                  })
                }
              />
              <YAxis tickLine={false} axisLine={false} tickMargin={8} domain={[0, 'auto']} />
              <ChartTooltip
                cursor={false}
                content={({ active, payload }) => {
                  if (!active || !payload || payload.length === 0) return null
                  const timestamp = payload[0].payload.timestamp
                  return (
                    <ChartTooltipContent
                      active={active}
                      payload={payload}
                      label={new Date(timestamp).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                      indicator="line"
                    />
                  )
                }}
              />
              <Line
                type="monotone"
                dataKey="messages_sent_rate"
                stroke="var(--color-messages_sent_rate)"
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
              <Line
                type="monotone"
                dataKey="messages_received_rate"
                stroke="var(--color-messages_received_rate)"
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
              <ChartLegend content={<ChartLegendContent />} />
            </LineChart>
          </ChartContainer>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Bandwidth</CardTitle>
          <CardDescription>Bytes per second</CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer config={bandwidthChartConfig} className="h-[400px] w-full">
            <LineChart data={rateData}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="timestamp"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                tickFormatter={(value) =>
                  new Date(value).toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                  })
                }
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                domain={[0, 'auto']}
                tickFormatter={(value) => formatBytesForAxis(value)}
              />
              <ChartTooltip
                cursor={false}
                content={({ active, payload }) => {
                  if (!active || !payload || payload.length === 0) return null
                  const timestamp = payload[0].payload.timestamp
                  return (
                    <ChartTooltipContent
                      active={active}
                      payload={payload}
                      label={new Date(timestamp).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                      indicator="line"
                    />
                  )
                }}
              />
              <Line
                type="monotone"
                dataKey="bytes_sent_rate"
                stroke="var(--color-bytes_sent_rate)"
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
              <Line
                type="monotone"
                dataKey="bytes_received_rate"
                stroke="var(--color-bytes_received_rate)"
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
              <ChartLegend content={<ChartLegendContent />} />
            </LineChart>
          </ChartContainer>
        </CardContent>
      </Card>
    </div>
  )
}

// Chart configurations
const messageChartConfig = {
  messages_sent_rate: {
    label: 'Sent',
    color: 'var(--chart-1)',
  },
  messages_received_rate: {
    label: 'Received',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

const bandwidthChartConfig = {
  bytes_sent_rate: {
    label: 'Sent',
    color: 'var(--chart-3)',
  },
  bytes_received_rate: {
    label: 'Received',
    color: 'var(--chart-4)',
  },
} satisfies ChartConfig

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  if (bytes < 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

function formatBytesForAxis(bytes: number): string {
  if (!bytes || bytes === 0) return '0'
  if (bytes < 0) return '0'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const value = Math.round(bytes / Math.pow(k, i))
  return `${value} ${sizes[i]}`
}

function formatBytesForTooltip(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  if (bytes < 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}
