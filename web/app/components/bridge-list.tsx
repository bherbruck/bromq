import { Network, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useDebounce } from '~/lib/use-debounce'
import { useAuth } from '~/lib/auth-context'
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import { DataTable } from '~/components/data-table'
import { DataTableColumnHeader } from '~/components/data-table-column-header'
import { BridgeForm } from '~/components/bridge-form'
import { api, type Bridge, type CreateBridgeRequest, type UpdateBridgeRequest } from '~/lib/api'

export function BridgeList() {
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()

  // Get pagination params from URL
  const page = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('pageSize') || '25', 10)
  const searchQuery = searchParams.get('search') || ''
  const sortBy = searchParams.get('sortBy') || 'created_at'
  const sortOrder = (searchParams.get('sortOrder') || 'desc') as 'asc' | 'desc'

  // Local search state for immediate UI updates
  const [localSearch, setLocalSearch] = useState(searchQuery)
  const debouncedSearch = useDebounce(localSearch, 300)

  // Dialog and form state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [editingBridge, setEditingBridge] = useState<Bridge | null>(null)
  const [deleteBridge, setDeleteBridge] = useState<Bridge | null>(null)
  const [formError, setFormError] = useState('')

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

  // Fetch bridges with React Query
  const { data: bridgesData, isLoading } = useQuery({
    queryKey: ['bridges', { page, pageSize, search: searchQuery, sortBy, sortOrder }],
    queryFn: () => api.getBridges({ page, pageSize, search: searchQuery, sortBy, sortOrder }),
  })

  const bridges = bridgesData?.data || []
  const pagination = bridgesData?.pagination || { total: 0, page: 1, page_size: 25, total_pages: 0 }

  const updateSearchParams = (updates: Record<string, string | number>) => {
    const newParams = new URLSearchParams(searchParams)
    Object.entries(updates).forEach(([key, value]) => {
      if (value === '' || value === 0) {
        newParams.delete(key)
      } else {
        newParams.set(key, String(value))
      }
    })
    setSearchParams(newParams)
  }

  // Pagination and sorting handlers for DataTable
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

  const canEdit = currentUser?.role === 'admin'

  // Mutations
  const createMutation = useMutation({
    mutationFn: (data: CreateBridgeRequest) => api.createBridge(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bridges'] })
      setIsCreateDialogOpen(false)
      setFormError('')
      updateSearchParams({ page: 1 })
    },
    onError: (err) => {
      setFormError(err instanceof Error ? err.message : 'Failed to create bridge')
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateBridgeRequest }) =>
      api.updateBridge(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bridges'] })
      setIsEditDialogOpen(false)
      setEditingBridge(null)
      setFormError('')
    },
    onError: (err) => {
      setFormError(err instanceof Error ? err.message : 'Failed to update bridge')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteBridge(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bridges'] })
      setDeleteBridge(null)
    },
    onError: (err) => {
      console.error('Failed to delete bridge:', err)
      setDeleteBridge(null)
    },
  })

  const handleCreate = (data: CreateBridgeRequest) => {
    setFormError('')
    createMutation.mutate(data)
  }

  const handleEditClick = (bridge: Bridge) => {
    setEditingBridge(bridge)
    setIsEditDialogOpen(true)
  }

  const handleUpdate = (data: UpdateBridgeRequest) => {
    if (!editingBridge) return
    setFormError('')
    updateMutation.mutate({ id: editingBridge.id, data })
  }

  const handleDelete = () => {
    if (!deleteBridge) return
    deleteMutation.mutate(deleteBridge.id)
  }

  const columns = useMemo<ColumnDef<Bridge>[]>(
    () => [
      {
        accessorKey: 'name',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Name" />,
        cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
      },
      {
        accessorKey: 'remote_host',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Remote Host" />,
        cell: ({ row }) => (
          <span className="font-mono text-sm">
            {row.original.remote_host}:{row.original.remote_port}
          </span>
        ),
      },
      {
        accessorKey: 'topics',
        header: 'Topics',
        cell: ({ row }) => {
          const topicsCount = row.original.topics?.length || 0
          return <span className="text-muted-foreground">{topicsCount} topic{topicsCount !== 1 ? 's' : ''}</span>
        },
        enableSorting: false,
      },
      {
        id: 'source',
        header: 'Source',
        cell: ({ row }) =>
          row.original.provisioned_from_config ? (
            <Badge variant="secondary" className="bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
              Provisioned
            </Badge>
          ) : (
            <span className="text-muted-foreground text-sm">Manual</span>
          ),
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Created" />,
        cell: ({ row }) => {
          const date = new Date(row.original.created_at)
          return <span className="text-muted-foreground text-sm">{date.toLocaleDateString()}</span>
        },
      },
      ...(canEdit
        ? [
            {
              id: 'actions',
              header: 'Actions',
              cell: ({ row }) => {
                const bridge = row.original
                return (
                  <div className="flex justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation()
                        handleEditClick(bridge)
                      }}
                      disabled={bridge.provisioned_from_config}
                      title={bridge.provisioned_from_config ? 'Edit config file to modify' : 'Edit bridge'}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation()
                        setDeleteBridge(bridge)
                      }}
                      disabled={bridge.provisioned_from_config}
                      title={bridge.provisioned_from_config ? 'Remove from config file to delete' : 'Delete bridge'}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                )
              },
            } as ColumnDef<Bridge>,
          ]
        : []),
    ],
    [canEdit]
  )

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-12">
        <Spinner className="h-8 w-8" />
        <p className="text-muted-foreground mt-4 text-sm">Loading bridges...</p>
      </div>
    )
  }

  if (bridges.length === 0 && !searchQuery) {
    return (
      <>
        <PageHeader title={<PageTitle>Bridges</PageTitle>} />
        <Empty>
          <EmptyMedia>
            <Network className="h-16 w-16 text-muted-foreground/50" />
          </EmptyMedia>
          <EmptyHeader>
            <EmptyTitle>No bridges configured</EmptyTitle>
            <EmptyDescription>
              Bridges connect this MQTT broker to remote brokers for message forwarding.
              <br />
              Configure bridges in your YAML config file.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </>
    )
  }

  return (
    <>
      <PageHeader
        title={<PageTitle>Bridges</PageTitle>}
        action={
          canEdit ? (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Create Bridge
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        data={bridges}
        searchColumn="name"
        searchPlaceholder="Search by name or host..."
        pageCount={pagination.total_pages}
        pagination={{ pageIndex: page - 1, pageSize }}
        sorting={sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []}
        columnFilters={localSearch ? [{ id: 'name', value: localSearch }] : []}
        manualPagination
        manualSorting
        manualFiltering
        onPaginationChange={setPaginationState}
        onSortingChange={setSortingState}
        onGlobalFilterChange={setGlobalFilter}
        onRowClick={(bridge) => navigate(`/bridges/${bridge.id}`)}
      />

      {/* Create Bridge Dialog */}
      <Dialog
        open={isCreateDialogOpen}
        onOpenChange={(open) => {
          setIsCreateDialogOpen(open)
          if (!open) setFormError('')
        }}
      >
        <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Create Bridge</DialogTitle>
            <DialogDescription>
              Create a new MQTT bridge to connect to a remote broker
            </DialogDescription>
          </DialogHeader>
          <BridgeForm
            mode="create"
            onSubmit={handleCreate}
            isSubmitting={createMutation.isPending}
            error={formError}
          />
        </DialogContent>
      </Dialog>

      {/* Edit Bridge Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) {
            setEditingBridge(null)
            setFormError('')
          }
        }}
      >
        <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Edit Bridge</DialogTitle>
            <DialogDescription>
              Update the bridge configuration
            </DialogDescription>
          </DialogHeader>
          {editingBridge && (
            <BridgeForm
              mode="edit"
              initialData={editingBridge}
              onSubmit={handleUpdate}
              isSubmitting={updateMutation.isPending}
              error={formError}
            />
          )}
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteBridge} onOpenChange={() => setDeleteBridge(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Bridge</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the bridge <strong>{deleteBridge?.name}</strong>?
              This will stop the bridge connection and remove all topic mappings.
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
