import { ArrowRight, Users, UserX } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { Route } from './+types/clients'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '~/components/ui/alert-dialog'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { api, type Client } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'Connected Clients - MQTT Server' }]

export default function ClientsPage() {
  const [clients, setClients] = useState<Client[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedClient, setSelectedClient] = useState<Client | null>(null)

  const fetchClients = async () => {
    try {
      const data = await api.getClients()
      setClients(data)
    } catch (error) {
      console.error('Failed to fetch clients:', error)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchClients()
    const interval = setInterval(fetchClients, 3000) // Refresh every 3 seconds

    return () => clearInterval(interval)
  }, [])

  const handleDisconnect = async () => {
    if (!selectedClient) return

    try {
      await api.disconnectClient(selectedClient.id)
      setSelectedClient(null)
      fetchClients()
    } catch (error) {
      console.error('Failed to disconnect client:', error)
    }
  }

  const getProtocolVersion = (version: number) => {
    const versions: Record<number, string> = {
      3: 'MQTT 3.1',
      4: 'MQTT 3.1.1',
      5: 'MQTT 5',
    }
    return versions[version] || `v${version}`
  }

  const formatConnectionDuration = (connectedAt?: number) => {
    if (!connectedAt) return 'Unknown'

    const now = Date.now() / 1000 // Convert to seconds
    const durationSec = Math.floor(now - connectedAt)

    if (durationSec < 60) return `${durationSec}s`

    const minutes = Math.floor(durationSec / 60)
    if (minutes < 60) return `${minutes}m`

    const hours = Math.floor(minutes / 60)
    const remainingMinutes = minutes % 60
    if (hours < 24) {
      return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`
    }

    const days = Math.floor(hours / 24)
    const remainingHours = hours % 24
    return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading clients...
      </div>
    )
  }

  return (
    <div>
      <PageHeader
        title={
          <PageTitle
            help={
              <>
                <p>
                  Shows all MQTT clients currently connected to the broker in real-time.
                </p>
                <p>
                  View connection details like protocol version, active subscriptions, and
                  connection duration. You can forcefully disconnect clients if needed.
                </p>
              </>
            }
          >
            Active Connections
          </PageTitle>
        }
      />

      {clients.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Users />
            </EmptyMedia>
            <EmptyTitle>No connected clients</EmptyTitle>
            <EmptyDescription>
              Clients will appear here when they connect to the MQTT broker
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Client ID</TableHead>
                <TableHead>Username</TableHead>
                <TableHead>Protocol</TableHead>
                <TableHead className="text-center">Subscriptions</TableHead>
                <TableHead className="text-center">In-Flight</TableHead>
                <TableHead>Connected</TableHead>
                <TableHead className="text-right" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {clients.map((client) => (
                <TableRow key={client.id}>
                  <TableCell className="font-mono text-sm">{client.id}</TableCell>
                  <TableCell>
                    {client.username || <span className="text-muted-foreground">anonymous</span>}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{getProtocolVersion(client.protocol_version)}</Badge>
                  </TableCell>
                  <TableCell className="text-center">
                    {client.subscriptions_count > 0 ? (
                      <Badge>{client.subscriptions_count}</Badge>
                    ) : (
                      <span className="text-muted-foreground">0</span>
                    )}
                  </TableCell>
                  <TableCell className="text-center">
                    {client.inflight_count > 0 ? (
                      <Badge variant="secondary">{client.inflight_count}</Badge>
                    ) : (
                      <span className="text-muted-foreground">0</span>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {formatConnectionDuration(client.connected_at)}
                  </TableCell>
                  <TableCell className="text-right space-x-2">
                    <Button variant="outline" size="sm" asChild>
                      <Link to={`/clients/${client.id}`}>
                        <ArrowRight className="h-4 w-4" />
                      </Link>
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setSelectedClient(client)}
                    >
                      <UserX className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Disconnect Confirmation Dialog */}
      <AlertDialog open={!!selectedClient} onOpenChange={() => setSelectedClient(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Disconnect Client</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to forcefully disconnect client{' '}
              <span className="font-mono">{selectedClient?.id}</span>? This action will terminate
              the client's connection immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDisconnect} className="bg-destructive">
              Disconnect
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
