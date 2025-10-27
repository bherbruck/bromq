import { Pencil, Plus, Shield, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import { DataTable } from '~/components/data-table'
import { DataTableColumnHeader } from '~/components/data-table-column-header'
import { api, type ACLRule, type MQTTUser } from '~/lib/api'

interface ACLRulesProps {
  mqttUserId?: number // Optional: filter rules by MQTT user ID
  showHeader?: boolean // Show page header (default: true)
  showMQTTUserColumn?: boolean // Show MQTT user column in table (default: true)
}

export function ACLRules({
  mqttUserId,
  showHeader = true,
  showMQTTUserColumn = true,
}: ACLRulesProps) {
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()

  // Always call useSearchParams (Rules of Hooks), but only use it when not embedded
  const [searchParams, setSearchParams] = useSearchParams()
  const shouldUseUrlParams = !mqttUserId // Only use URL params in standalone mode

  // Get pagination params from URL (or use defaults for embedded mode)
  const page = shouldUseUrlParams ? parseInt(searchParams.get('page') || '1', 10) : 1
  const pageSize = shouldUseUrlParams ? parseInt(searchParams.get('pageSize') || '25', 10) : 25
  const searchQuery = shouldUseUrlParams ? searchParams.get('search') || '' : ''
  const sortBy = shouldUseUrlParams ? searchParams.get('sortBy') || 'mqtt_user_id' : 'mqtt_user_id'
  const sortOrder = shouldUseUrlParams ? (searchParams.get('sortOrder') || 'asc') as 'asc' | 'desc' : 'asc'

  // Local search state for immediate UI updates
  const [localSearch, setLocalSearch] = useState(searchQuery)
  const debouncedSearch = useDebounce(localSearch, 300)

  // Dialog and form state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<ACLRule | null>(null)
  const [deleteRule, setDeleteRule] = useState<ACLRule | null>(null)
  const [selectedMqttUserId, setSelectedMqttUserId] = useState<number>(mqttUserId || 0)
  const [topicPattern, setTopicPattern] = useState('')
  const [permission, setPermission] = useState<'pub' | 'sub' | 'pubsub'>('pubsub')
  const [error, setError] = useState('')

  // Update URL when debounced search changes (only in non-embedded mode)
  useEffect(() => {
    if (shouldUseUrlParams && debouncedSearch !== searchQuery) {
      updateSearchParams({ search: debouncedSearch, page: 1 })
    }
  }, [debouncedSearch, shouldUseUrlParams, searchQuery])

  // Sync local search with URL on mount or navigation
  useEffect(() => {
    setLocalSearch(searchQuery)
  }, [searchQuery])

  // Fetch ACL rules with React Query
  const { data: rulesData, isLoading: rulesLoading } = useQuery({
    queryKey: ['acl-rules', { page, pageSize, search: searchQuery, sortBy, sortOrder, mqttUserId }],
    queryFn: () => api.getACLRules({ page, pageSize, search: searchQuery, sortBy, sortOrder }),
  })

  // Fetch MQTT users for dropdown
  const { data: usersData } = useQuery({
    queryKey: ['mqtt-users-all'],
    queryFn: () => api.getMQTTUsers({ pageSize: 1000 }),
  })

  const mqttUsers = usersData?.data || []

  // Filter rules by MQTT user if embedded
  const rules = mqttUserId && rulesData?.data
    ? rulesData.data.filter((rule) => rule.mqtt_user_id === mqttUserId)
    : rulesData?.data || []

  const pageCount = rulesData?.pagination?.total_pages || 0
  const isLoading = rulesLoading

  // Set default selected user ID
  useEffect(() => {
    if (!mqttUserId && mqttUsers.length > 0 && selectedMqttUserId === 0) {
      setSelectedMqttUserId(mqttUsers[0].id)
    } else if (mqttUserId && selectedMqttUserId === 0) {
      setSelectedMqttUserId(mqttUserId)
    }
  }, [mqttUsers, mqttUserId])

  // Update selectedMqttUserId when mqttUserId prop changes
  useEffect(() => {
    if (mqttUserId) {
      setSelectedMqttUserId(mqttUserId)
    }
  }, [mqttUserId])

  // URL params helper (only used in non-embedded mode)
  const updateSearchParams = (updates: Record<string, string | number>) => {
    if (!shouldUseUrlParams) return
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

  // Pagination handlers (only used in non-embedded mode)
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

  // Mutations
  const createMutation = useMutation({
    mutationFn: (data: { mqtt_user_id: number; topic_pattern: string; permission: 'pub' | 'sub' | 'pubsub' }) =>
      api.createACLRule(data.mqtt_user_id, data.topic_pattern, data.permission),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['acl-rules'] })
      setIsCreateDialogOpen(false)
      resetForm()
      if (shouldUseUrlParams) updateSearchParams({ page: 1 })
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to create ACL rule')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (data: { id: number; topic_pattern: string; permission: 'pub' | 'sub' | 'pubsub' }) =>
      api.updateACLRule(data.id, data.topic_pattern, data.permission),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['acl-rules'] })
      setIsEditDialogOpen(false)
      setEditingRule(null)
      resetForm()
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to update ACL rule')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteACLRule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['acl-rules'] })
      setDeleteRule(null)
    },
    onError: (err) => {
      console.error('Failed to delete ACL rule:', err)
      setDeleteRule(null)
    },
  })

  const resetForm = () => {
    setTopicPattern('')
    setPermission('pubsub')
    if (mqttUserId) {
      setSelectedMqttUserId(mqttUserId)
    } else if (mqttUsers.length > 0) {
      setSelectedMqttUserId(mqttUsers[0].id)
    }
    setError('')
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    createMutation.mutate({ mqtt_user_id: selectedMqttUserId, topic_pattern: topicPattern, permission })
  }

  const handleEditClick = (rule: ACLRule) => {
    setEditingRule(rule)
    setTopicPattern(rule.topic_pattern)
    setPermission(rule.permission as 'pub' | 'sub' | 'pubsub')
    setIsEditDialogOpen(true)
  }

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingRule) return
    setError('')
    updateMutation.mutate({ id: editingRule.id, topic_pattern: topicPattern, permission })
  }

  const handleDelete = () => {
    if (!deleteRule) return
    deleteMutation.mutate(deleteRule.id)
  }

  const getMQTTUsernameById = (id: number) => {
    return mqttUsers.find((u) => u.id === id)?.username || 'Unknown'
  }

  const getPermissionBadge = (perm: string) => {
    const variants: Record<string, 'default' | 'secondary' | 'outline'> = {
      pub: 'default',
      sub: 'secondary',
      pubsub: 'outline',
    }
    return <Badge variant={variants[perm] || 'outline'}>{perm}</Badge>
  }

  const canEdit = currentUser?.role === 'admin'

  // Define columns for DataTable
  const columns = useMemo<ColumnDef<ACLRule>[]>(
    () => [
      ...(showMQTTUserColumn
        ? [
            {
              accessorKey: 'mqtt_user_id',
              header: ({ column }) => <DataTableColumnHeader column={column} title="MQTT User" />,
              cell: ({ row }) => <span className="font-medium">{getMQTTUsernameById(row.getValue('mqtt_user_id'))}</span>,
            } as ColumnDef<ACLRule>,
          ]
        : []),
      {
        accessorKey: 'topic_pattern',
        header: ({ column }) => <DataTableColumnHeader column={column} title="Topic Pattern" />,
        cell: ({ row }) => <span className="font-mono text-sm">{row.getValue('topic_pattern')}</span>,
      },
      {
        accessorKey: 'permission',
        header: 'Permission',
        cell: ({ row }) => getPermissionBadge(row.getValue('permission')),
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
      ...(canEdit
        ? [
            {
              id: 'actions',
              cell: ({ row }) => {
                const rule = row.original
                return (
                  <div className="flex justify-end gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleEditClick(rule)}
                      disabled={rule.provisioned_from_config}
                      title={rule.provisioned_from_config ? 'Edit config file to modify' : 'Edit rule'}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setDeleteRule(rule)}
                      disabled={rule.provisioned_from_config}
                      title={rule.provisioned_from_config ? 'Remove from config file to delete' : 'Delete rule'}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                )
              },
            } as ColumnDef<ACLRule>,
          ]
        : []),
    ],
    [canEdit, showMQTTUserColumn, mqttUsers],
  )

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading ACL rules...
      </div>
    )
  }

  return (
    <div>
      {showHeader && (
        <PageHeader
          title={
            <PageTitle
              help={
                <>
                  <div>
                    <p className="font-semibold mb-1">Wildcards:</p>
                    <ul className="ml-4 space-y-1 list-disc">
                      <li>
                        <code className="bg-muted rounded px-1">+</code> - Single level wildcard (e.g.,{' '}
                        <code className="bg-muted rounded px-1">sensor/+/temp</code>)
                      </li>
                      <li>
                        <code className="bg-muted rounded px-1">#</code> - Multi-level wildcard (e.g.,{' '}
                        <code className="bg-muted rounded px-1">device/#</code>)
                      </li>
                    </ul>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">Permissions:</p>
                    <ul className="ml-4 space-y-1 list-disc">
                      <li><strong>pub</strong> - Publish only</li>
                      <li><strong>sub</strong> - Subscribe only</li>
                      <li><strong>pubsub</strong> - Both publish and subscribe</li>
                    </ul>
                  </div>
                </>
              }
            >
              Access Control Rules
            </PageTitle>
          }
          action={
            canEdit && mqttUsers.length > 0 ? (
              <Button onClick={() => setIsCreateDialogOpen(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Add Rule
              </Button>
            ) : undefined
          }
        />
      )}

      {mqttUsers.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Shield />
            </EmptyMedia>
            <EmptyTitle>No MQTT users available</EmptyTitle>
            <EmptyDescription>
              Create MQTT credentials first before adding ACL rules
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : rules.length === 0 && !localSearch && rulesData?.pagination?.total === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Shield />
            </EmptyMedia>
            <EmptyTitle>No ACL rules</EmptyTitle>
            <EmptyDescription>
              Add a rule to control topic access for MQTT users
            </EmptyDescription>
          </EmptyHeader>
          {canEdit && (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Add Rule
            </Button>
          )}
        </Empty>
      ) : mqttUserId ? (
        // Embedded mode (filtered by user) - no pagination UI
        <DataTable
          columns={columns}
          data={rules}
          getRowClassName={(row) => row.provisioned_from_config ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''}
        />
      ) : (
        // Standalone mode - with pagination
        <DataTable
          columns={columns}
          data={rules}
          searchColumn="topic_pattern"
          searchPlaceholder="Search topic patterns..."
          getRowClassName={(row) => row.provisioned_from_config ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''}
          pageCount={pageCount}
          pagination={{ pageIndex: page - 1, pageSize }}
          sorting={sortBy ? [{ id: sortBy, desc: sortOrder === 'desc' }] : []}
          columnFilters={localSearch ? [{ id: 'topic_pattern', value: localSearch }] : []}
          manualPagination
          manualSorting
          manualFiltering
          onPaginationChange={setPaginationState}
          onSortingChange={setSortingState}
          onGlobalFilterChange={setGlobalFilter}
        />
      )}

      {/* Create ACL Rule Dialog */}
      <Dialog
        open={isCreateDialogOpen}
        onOpenChange={(open) => {
          setIsCreateDialogOpen(open)
          if (!open) resetForm()
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create ACL Rule</DialogTitle>
            <DialogDescription>Define a new topic access control rule</DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="space-y-4 py-4">
              {/* Only show MQTT user selector if not filtering by user */}
              {!mqttUserId && (
                <Field>
                  <FieldLabel htmlFor="mqtt-user">MQTT User</FieldLabel>
                  <Select
                    value={selectedMqttUserId.toString()}
                    onValueChange={(v) => setSelectedMqttUserId(parseInt(v))}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {mqttUsers.map((user) => (
                        <SelectItem key={user.id} value={user.id.toString()}>
                          {user.username}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              )}
              <Field>
                <FieldLabel htmlFor="topic">Topic Pattern</FieldLabel>
                <Input
                  id="topic"
                  placeholder="e.g., sensor/+/temp or device/#"
                  value={topicPattern}
                  onChange={(e) => setTopicPattern(e.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="permission">Permission</FieldLabel>
                <Select
                  value={permission}
                  onValueChange={(v) => setPermission(v as 'pub' | 'sub' | 'pubsub')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="pub">Publish Only</SelectItem>
                    <SelectItem value="sub">Subscribe Only</SelectItem>
                    <SelectItem value="pubsub">Publish & Subscribe</SelectItem>
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
                {createMutation.isPending && <Spinner className="mr-2" />}
                {createMutation.isPending ? 'Creating...' : 'Create Rule'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit ACL Rule Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) {
            setEditingRule(null)
            resetForm()
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit ACL Rule</DialogTitle>
            <DialogDescription>
              Editing rule for {editingRule && getMQTTUsernameById(editingRule.mqtt_user_id)}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleUpdate}>
            <div className="space-y-4 py-4">
              <Field>
                <FieldLabel htmlFor="edit-topic">Topic Pattern</FieldLabel>
                <Input
                  id="edit-topic"
                  placeholder="e.g., sensor/+/temp or device/#"
                  value={topicPattern}
                  onChange={(e) => setTopicPattern(e.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="edit-permission">Permission</FieldLabel>
                <Select
                  value={permission}
                  onValueChange={(v) => setPermission(v as 'pub' | 'sub' | 'pubsub')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="pub">Publish Only</SelectItem>
                    <SelectItem value="sub">Subscribe Only</SelectItem>
                    <SelectItem value="pubsub">Publish & Subscribe</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <FieldError>{error}</FieldError>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsEditDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={updateMutation.isPending}>
                {updateMutation.isPending && <Spinner className="mr-2" />}
                {updateMutation.isPending ? 'Updating...' : 'Update Rule'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteRule} onOpenChange={() => setDeleteRule(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete ACL Rule</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this ACL rule? MQTT user{' '}
              <strong>{deleteRule && getMQTTUsernameById(deleteRule.mqtt_user_id)}</strong> will lose access
              to topic pattern <code className="font-mono">{deleteRule?.topic_pattern}</code>.
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
