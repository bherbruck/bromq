import { ChevronRight, Pencil, Plus, Trash2, Users } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import type { Route } from './+types/mqtt-users'
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
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
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
import { Spinner } from '~/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { Textarea } from '~/components/ui/textarea'
import { api, type MQTTUser } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'MQTT Users - MQTT Server' }]

export default function MQTTUsersPage() {
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<MQTTUser[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [deleteUser, setDeleteUser] = useState<MQTTUser | null>(null)
  const [editingUser, setEditingUser] = useState<MQTTUser | null>(null)

  // Form state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [description, setDescription] = useState('')
  const [isChangingPassword, setIsChangingPassword] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState('')

  const fetchUsers = async () => {
    try {
      const data = await api.getMQTTUsers()
      setUsers(data)
    } catch (error) {
      console.error('Failed to fetch MQTT users:', error)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchUsers()
  }, [])

  const resetForm = () => {
    setUsername('')
    setPassword('')
    setDescription('')
    setIsChangingPassword(false)
    setError('')
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsSubmitting(true)

    try {
      await api.createMQTTUser(username, password, description)
      setIsCreateDialogOpen(false)
      resetForm()
      fetchUsers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create MQTT user')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUser) return

    setError('')
    setIsSubmitting(true)

    try {
      // Update username and description
      await api.updateMQTTUser(editingUser.id, username, description)

      // Update password if changed
      if (isChangingPassword && password) {
        await api.updateMQTTUserPassword(editingUser.id, password)
      }

      setIsEditDialogOpen(false)
      setEditingUser(null)
      resetForm()
      fetchUsers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update MQTT user')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteUser) return

    try {
      await api.deleteMQTTUser(deleteUser.id)
      setDeleteUser(null)
      fetchUsers()
    } catch (error) {
      console.error('Failed to delete MQTT user:', error)
    }
  }

  const openEditDialog = (user: MQTTUser) => {
    setEditingUser(user)
    setUsername(user.username)
    setDescription(user.description || '')
    setPassword('')
    setIsChangingPassword(false)
    setIsEditDialogOpen(true)
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading MQTT users...
      </div>
    )
  }

  const canEdit = currentUser?.role === 'admin'

  return (
    <div className="space-y-6">
      {canEdit && (
        <div className="flex items-center justify-end">
          <Button onClick={() => setIsCreateDialogOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Add MQTT User
          </Button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>MQTT Users</CardTitle>
          <CardDescription>
            {users.length} MQTT user{users.length !== 1 ? 's' : ''} configured - these are used
            to authenticate devices connecting to the MQTT broker
          </CardDescription>
        </CardHeader>
        <CardContent>
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
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Created</TableHead>
                  {canEdit && <TableHead className="text-right">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium font-mono">{user.username}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {user.description || <span className="italic">No description</span>}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(user.created_at).toLocaleDateString()}
                    </TableCell>
                    {canEdit && (
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button variant="outline" size="sm" onClick={() => openEditDialog(user)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="destructive" size="sm" onClick={() => setDeleteUser(user)}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="sm" asChild>
                            <Link to={`/mqtt-users/${user.id}`}>
                              <ChevronRight className="h-4 w-4" />
                            </Link>
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Help Card */}
      <Card>
        <CardHeader>
          <CardTitle>About MQTT Users</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <p>
            MQTT users are username/password combinations that devices use to authenticate
            when connecting to the MQTT broker.
          </p>
          <p>
            Click the arrow next to each user to view details and manage ACL rules.
          </p>
        </CardContent>
      </Card>

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
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Spinner className="mr-2" />}
                {isSubmitting ? 'Creating...' : 'Create MQTT User'}
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
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Spinner className="mr-2" />}
                {isSubmitting ? 'Updating...' : 'Save Changes'}
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
