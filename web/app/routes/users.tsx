import type { ColumnDef, PaginationState, SortingState } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { MoreVertical, Pencil, Plus, Trash2, Users as UsersIcon } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { Route } from './+types/users'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { api, type DashboardUser } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'Dashboard Users - BroMQ' }]

export default function UsersPage() {
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
    queryKey: ['dashboard-users', { page, pageSize, search: searchQuery, sortBy, sortOrder }],
    queryFn: () => api.getDashboardUsers({ page, pageSize, search: searchQuery, sortBy, sortOrder }),
  })

  const users = data?.data || []
  const pageCount = data?.pagination.total_pages || 0

  // Dialog state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [deleteUser, setDeleteUser] = useState<DashboardUser | null>(null)
  const [editingUser, setEditingUser] = useState<DashboardUser | null>(null)

  // Form state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'viewer' | 'admin'>('viewer')
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
    mutationFn: (data: { username: string; password: string; role: 'viewer' | 'admin' }) =>
      api.createDashboardUser(data.username, data.password, data.role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dashboard-users'] })
      setIsCreateDialogOpen(false)
      resetForm()
      updateSearchParams({ page: 1 })
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to create user')
    },
  })

  const updateMutation = useMutation({
    mutationFn: async (data: {
      id: number
      username: string
      role: 'viewer' | 'admin'
      password?: string
    }) => {
      await api.updateDashboardUser(data.id, data.username, data.role)
      if (data.password) {
        await api.updateDashboardUserPassword(data.id, data.password)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dashboard-users'] })
      setIsEditDialogOpen(false)
      setEditingUser(null)
      resetForm()
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to update user')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteDashboardUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dashboard-users'] })
      setDeleteUser(null)
    },
    onError: (err) => {
      console.error('Failed to delete user:', err)
      setDeleteUser(null)
    },
  })

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setRole('viewer')
    setIsChangingPassword(false)
    setError('')
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    createMutation.mutate({ username, password, role })
  }

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUser) return
    setError('')
    updateMutation.mutate({
      id: editingUser.id,
      username,
      role,
      password: isChangingPassword ? password : undefined,
    })
  }

  const handleDelete = () => {
    if (!deleteUser) return
    deleteMutation.mutate(deleteUser.id)
  }

  const openEditDialog = (user: DashboardUser) => {
    setEditingUser(user)
    setUsername(user.username)
    setRole(user.role)
    setPassword('')
    setIsChangingPassword(false)
    setIsEditDialogOpen(true)
  }

  const isAdmin = currentUser?.role === 'admin'

  const columns = useMemo<ColumnDef<DashboardUser>[]>(
    () => [
      {
        accessorKey: 'username',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Username" />,
        cell: ({ row }) => <span className="font-medium">{row.getValue('username')}</span>,
      },
      {
        accessorKey: 'role',
        header: 'Role',
        cell: ({ row }) => (
          <Badge variant={row.getValue('role') === 'admin' ? 'default' : 'secondary'}>
            {row.getValue('role')}
          </Badge>
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
      ...(isAdmin
        ? [
            {
              id: 'actions',
              cell: ({ row }: { row: any }) => {
                const user = row.original as DashboardUser
                return (
                  <div className="flex justify-end">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8">
                          <MoreVertical className="h-4 w-4" />
                          <span className="sr-only">Actions</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => openEditDialog(user)}>
                          <Pencil className="h-4 w-4" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          onClick={() => setDeleteUser(user)}
                          className="text-destructive focus:text-destructive"
                        >
                          <Trash2 className="h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                )
              },
            } as ColumnDef<DashboardUser>,
          ]
        : []),
    ],
    [isAdmin],
  )

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading users...
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
                  Dashboard users are accounts that can log in to this web interface to
                  manage the MQTT server.
                </p>
                <p>
                  <strong>Admin</strong> users have full access to manage all settings, while{' '}
                  <strong>Viewer</strong> users can only view data without making changes.
                </p>
              </>
            }
          >
            Dashboard Users
          </PageTitle>
        }
        action={
          isAdmin ? (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="h-4 w-4" />
              Add User
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        data={users}
        searchColumn="username"
        searchPlaceholder="Search users..."
        emptyState={
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <UsersIcon />
              </EmptyMedia>
              <EmptyTitle>No dashboard users</EmptyTitle>
              <EmptyDescription>
                Add a user to allow them to log in to the dashboard.
              </EmptyDescription>
            </EmptyHeader>
            {isAdmin && (
              <Button onClick={() => setIsCreateDialogOpen(true)}>
                <Plus className="h-4 w-4" />
                Add User
              </Button>
            )}
          </Empty>
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

      {/* Create User Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={(open) => {
        setIsCreateDialogOpen(open)
        if (!open) resetForm()
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Dashboard User</DialogTitle>
            <DialogDescription>Add a new user who can log in to the web dashboard</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="space-y-4 py-4">
              <Field>
                <FieldLabel htmlFor="create-username">Username</FieldLabel>
                <Input
                  id="create-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
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
                <FieldLabel htmlFor="create-role">Role</FieldLabel>
                <Select value={role} onValueChange={(v) => setRole(v as 'viewer' | 'admin')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Viewer (Read Only)</SelectItem>
                    <SelectItem value="admin">Admin (Full Access)</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <FieldError>{error}</FieldError>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending && <Spinner />}
                {createMutation.isPending ? 'Creating...' : 'Create User'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit User Dialog */}
      <Dialog open={isEditDialogOpen} onOpenChange={(open) => {
        setIsEditDialogOpen(open)
        if (!open) {
          setEditingUser(null)
          resetForm()
        }
      }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Edit User</DialogTitle>
            <DialogDescription>Update user information or password</DialogDescription>
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
                <FieldLabel htmlFor="edit-role">Role</FieldLabel>
                <Select value={role} onValueChange={(v) => setRole(v as 'viewer' | 'admin')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Viewer (Read Only)</SelectItem>
                    <SelectItem value="admin">Admin (Full Access)</SelectItem>
                  </SelectContent>
                </Select>
              </Field>

              {/* Password Change Section */}
              <div className="border-t pt-4 space-y-3">
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
                {updateMutation.isPending && <Spinner />}
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
            <AlertDialogTitle>Delete Dashboard User</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete user <strong>{deleteUser?.username}</strong>? This
              action cannot be undone and they will no longer be able to log into the web dashboard.
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
