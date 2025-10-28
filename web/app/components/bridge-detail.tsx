import { ArrowLeft, ArrowRight, ArrowRightLeft, Info, Network, Pencil, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router'
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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { Spinner } from '~/components/ui/spinner'
import { BridgeForm } from '~/components/bridge-form'
import { api, type Bridge, type UpdateBridgeRequest } from '~/lib/api'

interface BridgeDetailProps {
  bridgeId: number
}

export function BridgeDetail({ bridgeId }: BridgeDetailProps) {
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()

  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [formError, setFormError] = useState('')

  const { data: bridge, isLoading, error } = useQuery({
    queryKey: ['bridge', bridgeId],
    queryFn: () => api.getBridge(bridgeId),
  })

  const canEdit = currentUser?.role === 'admin'

  const updateMutation = useMutation({
    mutationFn: (data: UpdateBridgeRequest) => api.updateBridge(bridgeId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bridge', bridgeId] })
      queryClient.invalidateQueries({ queryKey: ['bridges'] })
      setIsEditDialogOpen(false)
      setFormError('')
    },
    onError: (err) => {
      setFormError(err instanceof Error ? err.message : 'Failed to update bridge')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteBridge(bridgeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bridges'] })
      navigate('/bridges')
    },
    onError: (err) => {
      console.error('Failed to delete bridge:', err)
      setIsDeleteDialogOpen(false)
    },
  })

  const handleUpdate = (data: UpdateBridgeRequest) => {
    setFormError('')
    updateMutation.mutate(data)
  }

  const handleDelete = () => {
    deleteMutation.mutate()
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-12">
        <Spinner className="h-8 w-8" />
        <p className="text-muted-foreground mt-4 text-sm">Loading bridge...</p>
      </div>
    )
  }

  if (error || !bridge) {
    return (
      <div className="flex flex-col items-center justify-center p-12">
        <Network className="text-muted-foreground/50 h-16 w-16" />
        <p className="text-muted-foreground mt-4 text-sm">Bridge not found</p>
        <Button variant="outline" className="mt-4" onClick={() => navigate('/bridges')}>
          Back to Bridges
        </Button>
      </div>
    )
  }

  const getDirectionIcon = (direction: string) => {
    switch (direction) {
      case 'in':
        return <ArrowLeft className="h-4 w-4" />
      case 'out':
        return <ArrowRight className="h-4 w-4" />
      case 'both':
        return <ArrowRightLeft className="h-4 w-4" />
      default:
        return null
    }
  }

  const getDirectionLabel = (direction: string) => {
    switch (direction) {
      case 'in':
        return 'Inbound (Remote → Local)'
      case 'out':
        return 'Outbound (Local → Remote)'
      case 'both':
        return 'Bidirectional'
      default:
        return direction
    }
  }

  return (
    <>
      <PageHeader
        title={
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" onClick={() => navigate('/bridges')}>
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <PageTitle>{bridge.name}</PageTitle>
            {bridge.provisioned_from_config && (
              <Badge variant="secondary" className="bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                Provisioned
              </Badge>
            )}
          </div>
        }
        action={
          canEdit && !bridge.provisioned_from_config ? (
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => setIsEditDialogOpen(true)}>
                <Pencil className="mr-2 h-4 w-4" />
                Edit
              </Button>
              <Button variant="destructive" onClick={() => setIsDeleteDialogOpen(true)}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete
              </Button>
            </div>
          ) : undefined
        }
      />

      {bridge.provisioned_from_config && (
        <div className="bg-muted/50 flex items-start gap-3 rounded-lg border p-4">
          <Info className="text-muted-foreground mt-0.5 h-5 w-5 flex-shrink-0" />
          <div>
            <p className="text-sm font-medium">Configuration File Managed</p>
            <p className="text-muted-foreground text-sm">
              This bridge is managed by the YAML configuration file and cannot be modified through the UI.
              To make changes, edit the config file and restart the server.
            </p>
          </div>
        </div>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Connection Details</CardTitle>
            <CardDescription>Remote broker connection settings</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <p className="text-muted-foreground text-sm">Remote Host</p>
              <p className="font-mono text-sm">{bridge.remote_host}:{bridge.remote_port}</p>
            </div>
            {bridge.remote_username && (
              <div>
                <p className="text-muted-foreground text-sm">Username</p>
                <p className="font-mono text-sm">{bridge.remote_username}</p>
              </div>
            )}
            <div>
              <p className="text-muted-foreground text-sm">Client ID</p>
              <p className="font-mono text-sm">{bridge.client_id || '(auto-generated)'}</p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-muted-foreground text-sm">Keep Alive</p>
                <p className="text-sm">{bridge.keep_alive}s</p>
              </div>
              <div>
                <p className="text-muted-foreground text-sm">Timeout</p>
                <p className="text-sm">{bridge.connection_timeout}s</p>
              </div>
            </div>
            <div>
              <p className="text-muted-foreground text-sm">Clean Session</p>
              <p className="text-sm">{bridge.clean_session ? 'Yes' : 'No'}</p>
            </div>
          </CardContent>
        </Card>

        {bridge.metadata && Object.keys(bridge.metadata).length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Metadata</CardTitle>
              <CardDescription>Additional bridge information</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {Object.entries(bridge.metadata).map(([key, value]) => (
                <div key={key}>
                  <p className="text-muted-foreground text-sm">{key}</p>
                  <p className="text-sm">{JSON.stringify(value)}</p>
                </div>
              ))}
            </CardContent>
          </Card>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Topic Mappings</CardTitle>
          <CardDescription>
            {bridge.topics.length} topic routing rule{bridge.topics.length !== 1 ? 's' : ''}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {bridge.topics.length === 0 ? (
            <p className="text-muted-foreground text-center text-sm">No topic mappings configured</p>
          ) : (
            <div className="space-y-3">
              {bridge.topics.map((topic) => (
                <div key={topic.id} className="bg-muted/50 rounded-lg border p-4">
                  <div className="mb-3 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      {getDirectionIcon(topic.direction)}
                      <span className="text-sm font-medium">{getDirectionLabel(topic.direction)}</span>
                    </div>
                    <Badge variant="outline" className="font-mono text-xs">
                      QoS {topic.qos}
                    </Badge>
                  </div>
                  <div className="space-y-2">
                    <div>
                      <p className="text-muted-foreground text-xs">Local Pattern</p>
                      <p className="font-mono text-sm">{topic.local_pattern}</p>
                    </div>
                    <div>
                      <p className="text-muted-foreground text-xs">Remote Pattern</p>
                      <p className="font-mono text-sm">{topic.remote_pattern}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Timestamps</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div>
            <p className="text-muted-foreground text-sm">Created</p>
            <p className="text-sm">{new Date(bridge.created_at).toLocaleString()}</p>
          </div>
          <div>
            <p className="text-muted-foreground text-sm">Last Updated</p>
            <p className="text-sm">{new Date(bridge.updated_at).toLocaleString()}</p>
          </div>
        </CardContent>
      </Card>

      {/* Edit Bridge Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) setFormError('')
        }}
      >
        <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Edit Bridge</DialogTitle>
            <DialogDescription>
              Update the bridge configuration
            </DialogDescription>
          </DialogHeader>
          <BridgeForm
            mode="edit"
            initialData={bridge}
            onSubmit={handleUpdate}
            isSubmitting={updateMutation.isPending}
            error={formError}
          />
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Bridge</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the bridge <strong>{bridge.name}</strong>?
              This will stop the bridge connection and remove all topic mappings.
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive">
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
