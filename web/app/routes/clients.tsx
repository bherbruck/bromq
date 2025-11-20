import type { ColumnDef } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Circle, MoreVertical, Trash2, UserX, Users } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import type { Route } from './+types/clients'
import { useAuth } from '~/lib/auth-context'
import { useDebounce } from '~/lib/use-debounce'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { ToggleGroup, ToggleGroupItem } from '~/components/ui/toggle-group'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import { DataTable } from '~/components/data-table'
import { DataTableColumnHeader } from '~/components/data-table-column-header'
import { api, type MQTTClient } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'MQTT Clients - BroMQ' }]

export default function ClientsPage() {
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  // Get filter and pagination params from URL
  const showActiveOnly = searchParams.get('filter') !== 'all'
  const page = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('pageSize') || '25', 10)
  const searchQuery = searchParams.get('search') || ''
  const sortBy = searchParams.get('sortBy') || 'last_seen'
  const sortOrder = (searchParams.get('sortOrder') || 'desc') as 'asc' | 'desc'

  // Local search state for immediate UI updates
  const [localSearch, setLocalSearch] = useState(searchQuery)
  const debouncedSearch = useDebounce(localSearch, 300)

  // Dialog state
  const [disconnectClient, setDisconnectClient] = useState<MQTTClient | null>(null)
  const [deleteClient, setDeleteClient] = useState<MQTTClient | null>(null)

  // Update URL when debounced search changes
  useEffect(() => {
    if (debouncedSearch !== searchQuery) {
      updateSearchParams({ search: debouncedSearch, page: 1 })
    }
  }, [debouncedSearch, searchQuery])

  // Sync local search with URL on mount or navigation
  useEffect(() => {
    setLocalSearch(searchQuery)
  }, [searchQuery])

  // Fetch clients with React Query
  const { data: clientsData, isLoading } = useQuery({
    queryKey: ['mqtt-clients', { page, pageSize, search: searchQuery, sortBy, sortOrder, active: showActiveOnly }],
    queryFn: () => api.getMQTTClients({ page, pageSize, search: searchQuery, sortBy, sortOrder, activeOnly: showActiveOnly }),
  })

  const clients = clientsData?.data || []
  const pageCount = clientsData?.pagination?.total_pages || 0
  const canEdit = currentUser?.role === 'admin'

  // URL params helper
  const updateSearchParams = (updates: Record<string, string | number>) => {
    const newParams = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value) {
        newParams.set(key, String(value))
      } else {
        newParams.delete(key)
      }
    })
    setSearchParams(newParams, { replace: true })
  }

  // Pagination handlers
  const setPaginationState = (updater: (old: { pageIndex: number; pageSize: number }) => { pageIndex: number; pageSize: number }) => {
    const current = { pageIndex: page - 1, pageSize }
    const next = updater(current)
    updateSearchParams({ page: next.pageIndex + 1, pageSize: next.pageSize })
  }

  const setSortingState = (updater: (old: Array<{ id: string; desc: boolean }>) => Array<{ id: string; desc: boolean }>) => {
    const current = sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []
    const next = updater(current)
    if (next.length > 0) {
      updateSearchParams({ sortBy: next[0].id, sortOrder: next[0].desc ? 'desc' : 'asc' })
    }
  }

  const setGlobalFilter = (filter: string) => {
    setLocalSearch(filter)
  }

  // Toggle filter
  const toggleFilter = () => {
    updateSearchParams({ filter: showActiveOnly ? 'all' : 'active', page: 1 })
  }

  // Mutations
  const disconnectMutation = useMutation({
    mutationFn: (clientId: string) => api.disconnectClient(clientId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-clients'] })
      setDisconnectClient(null)
    },
    onError: (err) => {
      console.error('Failed to disconnect client:', err)
      setDisconnectClient(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteMQTTClient(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-clients'] })
      setDeleteClient(null)
    },
    onError: (err) => {
      console.error('Failed to delete client:', err)
      setDeleteClient(null)
    },
  })

  const handleDisconnect = () => {
    if (!disconnectClient) return
    disconnectMutation.mutate(disconnectClient.client_id)
  }

  const handleDelete = () => {
    if (!deleteClient) return
    deleteMutation.mutate(deleteClient.id)
  }

  // Table columns
  const columns = useMemo<ColumnDef<MQTTClient>[]>(
    () => [
      {
        accessorKey: 'client_id',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Client ID" />,
        cell: ({ row }) => <span className="font-mono text-sm">{row.getValue('client_id')}</span>,
      },
      {
        id: 'status',
        header: 'Status',
        cell: ({ row }) => (
          <div className="flex items-center gap-2">
            {row.original.is_active ? (
              <>
                <Circle className="h-2 w-2 fill-green-500 text-green-500" />
                <span className="text-green-700 dark:text-green-400 text-sm">Online</span>
              </>
            ) : (
              <>
                <Circle className="h-2 w-2 fill-gray-400 text-gray-400" />
                <span className="text-muted-foreground text-sm">Offline</span>
              </>
            )}
          </div>
        ),
      },
      {
        accessorKey: 'last_seen',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Last Seen" />,
        cell: ({ row }) => {
          const date = new Date(row.getValue('last_seen'))
          return <span className="text-muted-foreground text-sm">{date.toLocaleString()}</span>
        },
      },
      {
        accessorKey: 'first_seen',
        header: ({ column }) => <DataTableColumnHeader column={column} title="First Seen" />,
        cell: ({ row }) => {
          const date = new Date(row.getValue('first_seen'))
          return <span className="text-muted-foreground text-sm">{date.toLocaleString()}</span>
        },
      },
      {
        id: 'actions',
        cell: ({ row }) => {
          const client = row.original
          const isActive = client.is_active

          return (
            <div className="flex justify-end">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <MoreVertical className="h-4 w-4" />
                    <span className="sr-only">Actions</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {canEdit && isActive && (
                    <>
                      <DropdownMenuItem
                        onClick={(e) => {
                          e.stopPropagation()
                          setDisconnectClient(client)
                        }}
                        className="text-destructive focus:text-destructive"
                      >
                        <UserX className="h-4 w-4" />
                        Disconnect
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                    </>
                  )}
                  {canEdit && (
                    <DropdownMenuItem
                      onClick={(e) => {
                        e.stopPropagation()
                        setDeleteClient(client)
                      }}
                      className="text-destructive focus:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete Record
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          )
        },
      },
    ],
    [canEdit],
  )

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
                  Track all MQTT clients that have connected to the broker.
                </p>
                <p>
                  View connection history, status, and manage client records.
                  Active clients can be forcefully disconnected if needed.
                </p>
              </>
            }
          >
            MQTT Clients
          </PageTitle>
        }
      />

      <DataTable
        columns={columns}
        data={clients}
        searchColumn="client_id"
        searchPlaceholder="Search by client ID..."
        onRowClick={(client) => navigate(`/clients/${client.client_id}`)}
        toolbarRightElement={
          <ToggleGroup
            type="single"
            value={showActiveOnly ? 'active' : 'all'}
            onValueChange={(value) => {
              if (value) updateSearchParams({ filter: value, page: 1 })
            }}
            variant="outline"
          >
            <ToggleGroupItem value="active">Active Only</ToggleGroupItem>
            <ToggleGroupItem value="all">All Clients</ToggleGroupItem>
          </ToggleGroup>
        }
        emptyState={
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <Users />
              </EmptyMedia>
              <EmptyTitle>No clients found</EmptyTitle>
              <EmptyDescription>
                {showActiveOnly
                  ? 'No clients are currently connected to the broker'
                  : 'No clients have connected to the broker yet'}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        }
        pageCount={pageCount}
        pagination={{ pageIndex: page - 1, pageSize }}
        sorting={sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []}
        columnFilters={localSearch ? [{ id: 'client_id', value: localSearch }] : []}
        manualPagination
        manualSorting
        manualFiltering
        onPaginationChange={setPaginationState}
        onSortingChange={setSortingState}
        onGlobalFilterChange={setGlobalFilter}
      />

      {/* Disconnect Confirmation Dialog */}
      <AlertDialog open={!!disconnectClient} onOpenChange={() => setDisconnectClient(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Disconnect Client</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to forcefully disconnect client{' '}
              <span className="font-mono">{disconnectClient?.client_id}</span>? This action will
              terminate the client's connection immediately.
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

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteClient} onOpenChange={() => setDeleteClient(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Client Record</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the tracking record for client{' '}
              <span className="font-mono">{deleteClient?.client_id}</span>? This will remove all
              historical connection data.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive">
              Delete Record
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
