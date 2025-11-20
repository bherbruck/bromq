import type { ColumnDef } from '@tanstack/react-table'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Code, MoreVertical, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import type { Route } from './+types/scripts'
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import { Input } from '~/components/ui/input'
import { Field, FieldLabel, FieldError } from '~/components/ui/field'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import { Spinner } from '~/components/ui/spinner'
import { Switch } from '~/components/ui/switch'
import { Textarea } from '~/components/ui/textarea'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu'
import { DataTable } from '~/components/data-table'
import { DataTableColumnHeader } from '~/components/data-table-column-header'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { api, type Script } from '~/lib/api'
import { SCRIPT_EXAMPLE_TEMPLATES } from '~/lib/script-types'
import { toast } from 'sonner'

export const meta: Route.MetaFunction = () => [{ title: 'Scripts - BroMQ' }]

export default function ScriptsPage() {
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  // Fetch scripts
  const { data, isLoading } = useQuery({
    queryKey: ['scripts'],
    queryFn: () => api.getScripts(),
  })

  const scripts = data?.data || []

  // Dialog state
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [deleteScript, setDeleteScript] = useState<Script | null>(null)

  // Create form state
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [template, setTemplate] = useState<keyof typeof SCRIPT_EXAMPLE_TEMPLATES>('blank')
  const [enabled, setEnabled] = useState(true)
  const [error, setError] = useState('')

  // Create mutation
  const createMutation = useMutation({
    mutationFn: (data: any) => api.createScript(data),
    onSuccess: (script) => {
      queryClient.invalidateQueries({ queryKey: ['scripts'] })
      toast.success('Script created successfully')
      setIsCreateDialogOpen(false)
      resetForm()
      navigate(`/scripts/${script.id}`)
    },
    onError: (err: any) => {
      setError(err.message || 'Failed to create script')
      toast.error(err.message || 'Failed to create script')
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteScript(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scripts'] })
      toast.success('Script deleted successfully')
      setDeleteScript(null)
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to delete script')
    },
  })

  // Toggle enabled mutation
  const toggleEnabledMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      api.updateScriptEnabled(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scripts'] })
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to update script')
    },
  })

  const resetForm = () => {
    setName('')
    setDescription('')
    setTemplate('blank')
    setEnabled(true)
    setError('')
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!name.trim()) {
      setError('Script name is required')
      return
    }

    createMutation.mutate({
      name,
      description,
      content: SCRIPT_EXAMPLE_TEMPLATES[template],
      enabled,
      triggers: [
        {
          type: 'on_publish',
          topic_filter: '#',
          priority: 100,
          enabled: true,
        },
      ],
    })
  }

  // Table columns
  const columns: ColumnDef<Script>[] = [
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title="Name" />,
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
      enableSorting: true,
    },
    {
      accessorKey: 'description',
      header: ({ column }) => <DataTableColumnHeader column={column} title="Description" />,
      cell: ({ row }) => (
        <span className="text-muted-foreground text-sm">{row.original.description || '-'}</span>
      ),
      enableSorting: true,
    },
    {
      accessorKey: 'triggers',
      header: 'Triggers',
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1">
          {row.original.triggers.map((trigger, idx) => (
            <Badge key={idx} variant="secondary" className="text-xs">
              {trigger.type}
              {trigger.topic_filter && `: ${trigger.topic_filter}`}
            </Badge>
          ))}
        </div>
      ),
    },
    {
      accessorKey: 'enabled',
      header: 'Enabled',
      cell: ({ row }) => {
        const isProvisioned = row.original.provisioned_from_config
        return (
          <Switch
            checked={row.original.enabled}
            disabled={isProvisioned || currentUser?.role !== 'admin'}
            onCheckedChange={(enabled) => {
              if (!isProvisioned && currentUser?.role === 'admin') {
                toggleEnabledMutation.mutate({ id: row.original.id, enabled })
              }
            }}
          />
        )
      },
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
        ) : null,
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const isProvisioned = row.original.provisioned_from_config
        const isAdmin = currentUser?.role === 'admin'

        if (!isAdmin) return null

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
                <DropdownMenuItem
                  onClick={(e) => {
                    e.stopPropagation()
                    navigate(`/scripts/${row.original.id}`)
                  }}
                >
                  <Code className="h-4 w-4" />
                  View
                </DropdownMenuItem>
                {!isProvisioned && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onClick={(e) => {
                        e.stopPropagation()
                        setDeleteScript(row.original)
                      }}
                      className="text-destructive focus:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )
      },
    },
  ]

  if (isLoading) {
    return (
      <div className="flex h-[calc(100vh-4rem)] items-center justify-center">
        <div className="flex flex-col items-center gap-2">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          <p className="text-sm text-muted-foreground">Loading scripts...</p>
        </div>
      </div>
    )
  }

  const canEdit = currentUser?.role === 'admin'

  return (
    <div>
      <PageHeader
        title={<PageTitle>Scripts</PageTitle>}
        action={
          canEdit ? (
            <Button onClick={() => setIsCreateDialogOpen(true)}>
              <Plus className="h-4 w-4" />
              Create Script
            </Button>
          ) : undefined
        }
      />

      <DataTable
        columns={columns}
        data={scripts || []}
        searchColumn="name"
        searchPlaceholder="Search scripts..."
        onRowClick={(script) => navigate(`/scripts/${script.id}`)}
        getRowClassName={(script) =>
          script.provisioned_from_config ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''
        }
        emptyState={
          <Empty className="mt-8">
            <EmptyHeader>
              <EmptyMedia>
                <Code className="h-12 w-12" />
              </EmptyMedia>
              <EmptyTitle>No scripts configured</EmptyTitle>
              <EmptyDescription>
                Scripts execute automatically in response to MQTT events. Get started by creating your
                first script.
              </EmptyDescription>
            </EmptyHeader>
            {canEdit && (
              <Button onClick={() => setIsCreateDialogOpen(true)}>
                <Plus className="h-4 w-4" />
                Create Script
              </Button>
            )}
          </Empty>
        }
      />

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
            <DialogTitle>Create Script</DialogTitle>
            <DialogDescription>
              Create a new JavaScript script to automate MQTT event handling
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="space-y-4 py-4">
              <Field>
                <FieldLabel htmlFor="create-name">Name *</FieldLabel>
                <Input
                  id="create-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-script"
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="create-description">Description (optional)</FieldLabel>
                <Textarea
                  id="create-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="What does this script do?"
                  rows={2}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="create-template">Starting Template</FieldLabel>
                <Select value={template} onValueChange={(val) => setTemplate(val as any)}>
                  <SelectTrigger id="create-template">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="blank">Blank Script</SelectItem>
                    <SelectItem value="message-logger">Message Logger</SelectItem>
                    <SelectItem value="temperature-alert">Temperature Alert</SelectItem>
                    <SelectItem value="connection-tracker">Connection Tracker</SelectItem>
                    <SelectItem value="rate-limiter">Rate Limiter</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  You can edit the code after creation
                </p>
              </Field>
              <Field>
                <div className="flex items-center justify-between">
                  <FieldLabel htmlFor="create-enabled">Enabled</FieldLabel>
                  <Switch
                    id="create-enabled"
                    checked={enabled}
                    onCheckedChange={setEnabled}
                  />
                </div>
              </Field>
              <FieldError>{error}</FieldError>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending && <Spinner />}
                {createMutation.isPending ? 'Creating...' : 'Create Script'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <AlertDialog open={!!deleteScript} onOpenChange={() => setDeleteScript(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Script</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete "{deleteScript?.name}"? This will also delete all logs
              and state for this script. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteScript && deleteMutation.mutate(deleteScript.id)}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
