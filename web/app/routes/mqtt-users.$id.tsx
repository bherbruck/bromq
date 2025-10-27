import { ArrowLeft, Save, Trash2, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import type { Route } from './+types/mqtt-users.$id'
import { useAuth } from '~/lib/auth-context'
import { ACLRules } from '~/components/acl-rules'
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
import { Input } from '~/components/ui/input'
import { Label } from '~/components/ui/label'
import { Field, FieldLabel, FieldError } from '~/components/ui/field'
import { Spinner } from '~/components/ui/spinner'
import { Textarea } from '~/components/ui/textarea'
import { api, type MQTTUser } from '~/lib/api'

export const meta: Route.MetaFunction = ({ params }) => [
  { title: `MQTT User #${params.id} - MQTT Server` },
]

export default function MQTTUserDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)

  // Form state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [description, setDescription] = useState('')
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const [isSaved, setIsSaved] = useState(true)
  const [error, setError] = useState('')

  // Fetch user with React Query
  const { data: mqttUser, isLoading } = useQuery({
    queryKey: ['mqtt-user', id],
    queryFn: () => api.getMQTTUser(parseInt(id!)),
    enabled: !!id,
  })

  // Initialize form when user data loads
  useEffect(() => {
    if (mqttUser) {
      setUsername(mqttUser.username)
      setDescription(mqttUser.description || '')
      setIsSaved(true)
    }
  }, [mqttUser])

  // Track if form is dirty
  useEffect(() => {
    if (!mqttUser) return
    const isChanged =
      username !== mqttUser.username ||
      description !== (mqttUser.description || '') ||
      isChangingPassword
    setIsSaved(!isChanged)
  }, [username, description, isChangingPassword, mqttUser])

  const handleCancel = () => {
    if (mqttUser) {
      setUsername(mqttUser.username)
      setDescription(mqttUser.description || '')
      setPassword('')
      setIsChangingPassword(false)
      setError('')
      setIsSaved(true)
    }
  }

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: async (data: { id: number; username: string; description: string; password?: string }) => {
      await api.updateMQTTUser(data.id, data.username, data.description)
      if (data.password) {
        await api.updateMQTTUserPassword(data.id, data.password)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-user', id] })
      queryClient.invalidateQueries({ queryKey: ['mqtt-users'] })
      setPassword('')
      setIsChangingPassword(false)
      setIsSaved(true)
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to update MQTT user')
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.deleteMQTTUser(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mqtt-users'] })
      navigate('/mqtt-users')
    },
    onError: (err) => {
      setError(err instanceof Error ? err.message : 'Failed to delete MQTT user')
      setIsDeleteDialogOpen(false)
    },
  })

  const handleSave = () => {
    if (!mqttUser) return
    setError('')
    updateMutation.mutate({
      id: mqttUser.id,
      username,
      description,
      password: isChangingPassword ? password : undefined,
    })
  }

  const handleDelete = () => {
    if (!mqttUser) return
    deleteMutation.mutate(mqttUser.id)
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading MQTT user...
      </div>
    )
  }

  if (!mqttUser) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        MQTT user not found
      </div>
    )
  }

  const canEdit = currentUser?.role === 'admin'
  const isProvisioned = mqttUser.provisioned_from_config
  const canEditProvisioned = canEdit && !isProvisioned

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link to="/mqtt-users">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to MQTT Users
            </Link>
          </Button>
        </div>
        {canEdit && (
          <div className="flex gap-2">
            {!isSaved && !isProvisioned && (
              <>
                <Button variant="outline" onClick={handleCancel} disabled={updateMutation.isPending}>
                  <X className="mr-2 h-4 w-4" />
                  Cancel
                </Button>
                <Button onClick={handleSave} disabled={updateMutation.isPending}>
                  {updateMutation.isPending && <Spinner className="mr-2" />}
                  {!updateMutation.isPending && <Save className="mr-2 h-4 w-4" />}
                  {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
                </Button>
              </>
            )}
            <Button
              variant="destructive"
              onClick={() => setIsDeleteDialogOpen(true)}
              disabled={isProvisioned}
              title={isProvisioned ? "Remove from config file to delete" : "Delete user"}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete User
            </Button>
          </div>
        )}
      </div>

      {/* User Details Card */}
      <Card className={isProvisioned ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''}>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-2xl">MQTT User Details</CardTitle>
              <CardDescription>User #{mqttUser.id}</CardDescription>
            </div>
            <div className="flex gap-2">
              {isProvisioned && (
                <Badge variant="secondary" className="bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                  Provisioned
                </Badge>
              )}
              <Badge variant="outline">MQTT Credentials</Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          {isProvisioned && (
            <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-100">
              <p className="font-medium">This user is managed by the config file.</p>
              <p className="mt-1 text-blue-700 dark:text-blue-300">
                Edit the config file and restart the server to modify.
              </p>
            </div>
          )}

          <FieldError>{error}</FieldError>

          <Field>
            <FieldLabel htmlFor="username">Username</FieldLabel>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="device-001"
              disabled={!canEditProvisioned}
              className="font-mono"
            />
          </Field>

          <Field>
            <FieldLabel htmlFor="description">Description</FieldLabel>
            <Textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Temperature sensor in building A"
              rows={3}
              disabled={!canEditProvisioned}
            />
          </Field>

          {/* Password Change Section */}
          {canEditProvisioned && (
            <div className="space-y-4">
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
                  />
                </Field>
              )}
            </div>
          )}

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <Label className="text-muted-foreground">Created</Label>
              <p className="text-foreground mt-1">{new Date(mqttUser.created_at).toLocaleString()}</p>
            </div>
            <div>
              <Label className="text-muted-foreground">Last Updated</Label>
              <p className="text-foreground mt-1">{new Date(mqttUser.updated_at).toLocaleString()}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ACL Rules Section */}
      <div>
        <ACLRules
          mqttUserId={mqttUser.id}
          showMQTTUserColumn={false}
          showHeader={true}
        />
      </div>

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete MQTT User</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete MQTT user{' '}
              <strong className="font-mono">{mqttUser.username}</strong>? This will prevent any
              devices using this account from connecting to the MQTT broker. All associated
              ACL rules will also be deleted.
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
