import { ArrowLeft, Save, Trash2, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams, useNavigate } from 'react-router'
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
import { Separator } from '~/components/ui/separator'
import { Textarea } from '~/components/ui/textarea'
import { api, type MQTTUser } from '~/lib/api'

export const meta: Route.MetaFunction = ({ params }) => [
  { title: `MQTT User #${params.id} - MQTT Server` },
]

export default function MQTTUserDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const [mqttUser, setMqttUser] = useState<MQTTUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)

  // Form state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [description, setDescription] = useState('')
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isSaved, setIsSaved] = useState(true)
  const [error, setError] = useState('')

  const fetchUser = async () => {
    if (!id) return
    try {
      const users = await api.getMQTTUsers()
      const user = users.find((u) => u.id === parseInt(id))
      if (user) {
        setMqttUser(user)
        setUsername(user.username)
        setDescription(user.description || '')
        setIsSaved(true)
      } else {
        navigate('/mqtt-users')
      }
    } catch (error) {
      console.error('Failed to fetch MQTT user:', error)
      navigate('/mqtt-users')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchUser()
  }, [id])

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

  const handleSave = async () => {
    if (!mqttUser) return

    setError('')
    setIsSubmitting(true)

    try {
      // Update username and description
      await api.updateMQTTUser(mqttUser.id, username, description)

      // Update password if changed
      if (isChangingPassword && password) {
        await api.updateMQTTUserPassword(mqttUser.id, password)
      }

      setPassword('')
      setIsChangingPassword(false)
      await fetchUser()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update MQTT user')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!mqttUser) return

    try {
      await api.deleteMQTTUser(mqttUser.id)
      navigate('/mqtt-users')
    } catch (error) {
      console.error('Failed to delete MQTT user:', error)
    }
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
            {!isSaved && (
              <>
                <Button variant="outline" onClick={handleCancel} disabled={isSubmitting}>
                  <X className="mr-2 h-4 w-4" />
                  Cancel
                </Button>
                <Button onClick={handleSave} disabled={isSubmitting}>
                  {isSubmitting && <Spinner className="mr-2" />}
                  {!isSubmitting && <Save className="mr-2 h-4 w-4" />}
                  {isSubmitting ? 'Saving...' : 'Save Changes'}
                </Button>
              </>
            )}
            <Button variant="destructive" onClick={() => setIsDeleteDialogOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" />
              Delete User
            </Button>
          </div>
        )}
      </div>

      {/* User Details Card */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-2xl">MQTT User Details</CardTitle>
              <CardDescription>User #{mqttUser.id}</CardDescription>
            </div>
            <Badge variant="outline">MQTT Credentials</Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-6">
          <FieldError>{error}</FieldError>

          <Field>
            <FieldLabel htmlFor="username">Username</FieldLabel>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="device-001"
              disabled={!canEdit}
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
              disabled={!canEdit}
            />
          </Field>

          <Separator />

          {/* Password Change Section */}
          {canEdit && (
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

          <Separator />

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
          showHelp={true}
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
