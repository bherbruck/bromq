import type { ColumnDef, PaginationState, SortingState } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2, Users } from 'lucide-react'
import { useMemo, useState, useEffect } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { Route } from './+types/mqtt-users'
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import { Input } from '~/components/ui/input'
import { Field, FieldLabel, FieldError } from '~/components/ui/field'
import { DataTable } from '~/components/data-table'
import { DataTableColumnHeader } from '~/components/data-table-column-header'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import { Textarea } from '~/components/ui/textarea'
import { api, type MQTTUser } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'MQTT Users - MQTT Server' }]

export default function MQTTUsersPage() {
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

  // Update URL when debounced search changes
  useEffect(() => {
    if (debouncedSearch !== searchQuery) {
      updateSearchParams({ search: debouncedSearch, page: 1 })
    }
  }, [debouncedSearch])

  // Sync local search with URL on mount or navigation
  useEffect(() => {
    setLocalSearch(searchQuery)
  }, [searchQuery])

  // Fetch data with React Query
  const { data, isLoading } = useQuery({
    queryKey: ['mqtt-users', { page, pageSize, search: searchQuery, sortBy, sortOrder }],
    queryFn: () => api.getMQTTUsers({ page, pageSize, search: searchQuery, sortBy, sortOrder }),
  })

  const users = data?.data || []
  const pageCount = data?.pagination.total_pages || 0

  // Dialog state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [deleteUser, setDeleteUser] = useState<MQTTUser | null>(null)
  const [editingUser, setEditingUser] = useState<MQTTUser | null>(null)

  // Form state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [description, setDescription] = useState('')
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const [error, setError] = useState('')

  // Update URL params
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
  const setPaginationState = (updater: (old: PaginationState) => PaginationState) => {
    const current = { pageIndex: page - 1, pageSize }
    const next = updater(current)
    updateSearchParams({ page: next.pageIndex + 1, pageSize: next.pageSize })
  }

  const setSortingState = (updater: (old: SortingState) => SortingState) => {
    const current: SortingState = sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []
    const next = updater(current)
    if (next.length > 0) {
      updateSearchParams({ sortBy: next[0].id, sortOrder: next[0].desc ? 'desc' : 'asc' })
    }
  }

  const setGlobalFilter = (filter: string) => {
    setLocalSearch(filter) // Update local state immediately for UI responsiveness
  }

  // Mutations
  const createMutation = useMutation({
    mutationFn: (data: { username: string; password: string; description?: string }) =>
      api.createMQTTUser(data.username, data.password, data.description),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-users'] })
      setIsCreateDialogOpen(false)
      resetForm()
      updateSearchParams({ page: 1 })
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to create MQTT user')
    },
  })

  const updateMutation = useMutation({
    mutationFn: async (data: {
      id: number
      username: string
      description?: string
      password?: string
    }) => {
      await api.updateMQTTUser(data.id, data.username, data.description)
      if (data.password) {
        await api.updateMQTTUserPassword(data.id, data.password)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-users'] })
      setIsEditDialogOpen(false)
      setEditingUser(null)
      resetForm()
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to update MQTT user')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteMQTTUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-users'] })
      setDeleteUser(null)
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to delete MQTT user')
      setDeleteUser(null)
    },
  })

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setDescription('')
    setIsChangingPassword(false)
    setError('')
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    createMutation.mutate({ username, password, description })
  }

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUser) return
    setError('')
    updateMutation.mutate({
      id: editingUser.id,
      username,
      description,
      password: isChangingPassword ? password : undefined,
    })
  }

  const handleDelete = () => {
    if (!deleteUser) return
    deleteMutation.mutate(deleteUser.id)
  }

  const openEditDialog = (user: MQTTUser) => {
    setEditingUser(user)
    setUsername(user.username)
    setDescription(user.description || '')
    setPassword('')
    setIsChangingPassword(false)
    setIsEditDialogOpen(true)
  }

  const canEdit = currentUser?.role === 'admin'

  const columns = useMemo<ColumnDef<MQTTUser>[]>(
    () => [
      {
        accessorKey: 'username',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Username" />,
        cell: ({ row }) => (
          <Link
            to={`/mqtt-users/${row.original.id}`}
            className="hover:text-primary font-medium hover:underline"
          >
            {row.getValue('username')}
          </Link>
        ),
      },
      {
        accessorKey: 'description',
        header: 'Description',
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {row.getValue('description') || <span className="italic">No description</span>}
          </span>
        ),
      },
      {
        accessorKey: 'provisioned_from_config',
        header: 'Source',
        cell: ({ row }) =>
          row.getValue('provisioned_from_config') ? (
            <Badge
              variant="secondary"
              className="bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300"
            >
              Provisioned
            </Badge>
          ) : (
            <span className="text-muted-foreground text-sm">Manual</span>
          ),
      },
      {
        accessorKey: 'created_at',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Created" />,
        cell: ({ row }) => (
          <span className="text-muted-foreground">
            {new Date(row.getValue('created_at')).toLocaleDateString()}
          </span>
        ),
      },
      ...(canEdit
        ? [
            {
              id: 'actions',
              cell: ({ row }: { row: any }) => {
                const user = row.original as MQTTUser
                return (
                  <div className="flex justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => openEditDialog(user)}
                      disabled={user.provisioned_from_config}
                      title={
                        user.provisioned_from_config ? 'Edit config file to modify' : 'Edit user'
                      }
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setDeleteUser(user)}
                      disabled={user.provisioned_from_config}
                      title={
                        user.provisioned_from_config
                          ? 'Remove from config file to delete'
                          : 'Delete user'
                      }
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                )
              },
            } as ColumnDef<MQTTUser>,
          ]
        : []),
    ],
    [canEdit],
  )

  if (isLoading) {
    return (
      <div className="text-muted-foreground flex items-center gap-2">
        <Spinner />
        Loading MQTT users...
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
                  MQTT users are credentials that devices use to authenticate when connecting to the
                  MQTT broker.
                </p>
                <p>
                  Each MQTT user can have multiple devices connecting with the same credentials, and
                  you can control their topic access with ACL rules.
                </p>
              </>
            }
          >
            MQTT Users
          </PageTitle>
        }
        action={
          canEdit ? (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Add MQTT User
            </Button>
          ) : undefined
        }
      />

      {users.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Users />
            </EmptyMedia>
            <EmptyTitle>No MQTT users</EmptyTitle>
            <EmptyDescription>
              Add an MQTT user to allow devices to connect to the broker.
            </EmptyDescription>
          </EmptyHeader>
          {canEdit && (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Add MQTT User
            </Button>
          )}
        </Empty>
      ) : (
        <DataTable
          columns={columns}
          data={users}
          searchColumn="username"
          searchPlaceholder="Search users..."
          getRowClassName={(user) =>
            user.provisioned_from_config ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''
          }
          // Server-side pagination - pass controlled state
          pageCount={pageCount}
          pagination={{ pageIndex: page - 1, pageSize }}
          sorting={sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []}
          columnFilters={localSearch ? [{ id: 'username', value: localSearch }] : []}
          manualPagination
          manualSorting
          manualFiltering
          onPaginationChange={setPaginationState}
          onSortingChange={setSortingState}
          onGlobalFilterChange={setGlobalFilter}
        />
      )}

      {/* Create Dialog */}
      <Dialog
        open={isCreateDialogOpen}
        onOpenChange={(open) => {
          setIsCreateDialogOpen(open)
          if (!open) resetForm()
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create MQTT User</DialogTitle>
            <DialogDescription>
              Add a new MQTT user for a device or application to connect to the broker
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="space-y-4 py-4">
              <Field>
                <FieldLabel htmlFor="create-username">Username</FieldLabel>
                <Input
                  id="create-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="device-001"
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="create-password">Password</FieldLabel>
                <Input
                  id="create-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="create-description">Description (optional)</FieldLabel>
                <Textarea
                  id="create-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Temperature sensor in building A"
                  rows={3}
                />
              </Field>
              <FieldError>{error}</FieldError>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending && <Spinner className="mr-2" />}
                {createMutation.isPending ? 'Creating...' : 'Create MQTT User'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) {
            setEditingUser(null)
            resetForm()
          }
        }}
      >
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Edit MQTT User</DialogTitle>
            <DialogDescription>Update username, description, or password</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleUpdate}>
            <div className="space-y-4 py-4">
              <Field>
                <FieldLabel htmlFor="edit-username">Username</FieldLabel>
                <Input
                  id="edit-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="edit-description">Description</FieldLabel>
                <Textarea
                  id="edit-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Temperature sensor in building A"
                  rows={3}
                />
              </Field>

              {/* Password Change Section */}
              <div className="space-y-3 border-t pt-4">
                <div className="flex items-center space-x-2">
                  <input
                    type="checkbox"
                    id="change-password"
                    checked={isChangingPassword}
                    onChange={(e) => {
                      setIsChangingPassword(e.target.checked)
                      if (!e.target.checked) setPassword('')
                    }}
                    className="rounded border-gray-300"
                  />
                  <FieldLabel htmlFor="change-password" className="cursor-pointer font-medium">
                    Change password
                  </FieldLabel>
                </div>
                {isChangingPassword && (
                  <Field>
                    <FieldLabel htmlFor="new-password">New Password</FieldLabel>
                    <Input
                      id="new-password"
                      type="password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Enter new password"
                      required={isChangingPassword}
                    />
                  </Field>
                )}
              </div>

              <FieldError>{error}</FieldError>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsEditDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={updateMutation.isPending}>
                {updateMutation.isPending && <Spinner className="mr-2" />}
                {updateMutation.isPending ? 'Updating...' : 'Save Changes'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteUser} onOpenChange={() => setDeleteUser(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete MQTT User</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete MQTT user{' '}
              <strong className="font-mono">{deleteUser?.username}</strong>? This will prevent any
              devices using this account from connecting to the MQTT broker. All associated ACL
              rules will also be deleted.
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
    </div>
  )
}
