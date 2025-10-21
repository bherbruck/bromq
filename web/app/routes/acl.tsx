import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Route } from './+types/acl'
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '~/components/ui/dialog'
import { Input } from '~/components/ui/input'
import { Label } from '~/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { api, type ACLRule, type User } from '~/lib/api'

export const meta: Route.MetaFunction = () => [{ title: 'ACL Rules - MQTT Server' }]

export default function ACLPage() {
  const [rules, setRules] = useState<ACLRule[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [deleteRule, setDeleteRule] = useState<ACLRule | null>(null)

  // Form state
  const [userId, setUserId] = useState<number>(0)
  const [topicPattern, setTopicPattern] = useState('')
  const [permission, setPermission] = useState<'pub' | 'sub' | 'pubsub'>('pubsub')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState('')

  const fetchData = async () => {
    try {
      const [rulesData, usersData] = await Promise.all([api.getACLRules(), api.getUsers()])
      setRules(rulesData)
      setUsers(usersData)
      if (usersData.length > 0 && userId === 0) {
        setUserId(usersData[0].id)
      }
    } catch (error) {
      console.error('Failed to fetch data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const resetForm = () => {
    setTopicPattern('')
    setPermission('pubsub')
    if (users.length > 0) {
      setUserId(users[0].id)
    }
    setError('')
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsSubmitting(true)

    try {
      await api.createACLRule(userId, topicPattern, permission)
      setIsCreateDialogOpen(false)
      resetForm()
      fetchData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create ACL rule')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteRule) return

    try {
      await api.deleteACLRule(deleteRule.id)
      setDeleteRule(null)
      fetchData()
    } catch (error) {
      console.error('Failed to delete ACL rule:', error)
    }
  }

  const getUsernameById = (id: number) => {
    return users.find((u) => u.id === id)?.username || 'Unknown'
  }

  const getPermissionBadge = (perm: string) => {
    const variants: Record<string, 'default' | 'secondary' | 'outline'> = {
      pub: 'default',
      sub: 'secondary',
      pubsub: 'outline',
    }
    return <Badge variant={variants[perm] || 'outline'}>{perm}</Badge>
  }

  if (isLoading) {
    return <div className="text-muted-foreground">Loading ACL rules...</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-end">
        <Button onClick={() => setIsCreateDialogOpen(true)} disabled={users.length === 0}>
          <Plus className="mr-2 h-4 w-4" />
          Add Rule
        </Button>
      </div>

      {users.length === 0 ? (
        <Card>
          <CardContent className="py-8">
            <div className="text-muted-foreground text-center">
              <p className="mb-2">No users available</p>
              <p className="text-sm">Create users first before adding ACL rules</p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Access Control Rules</CardTitle>
            <CardDescription>
              {rules.length} rule{rules.length !== 1 ? 's' : ''} configured
            </CardDescription>
          </CardHeader>
          <CardContent>
            {rules.length === 0 ? (
              <div className="text-muted-foreground py-8 text-center">
                No ACL rules configured. Add a rule to control topic access.
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Topic Pattern</TableHead>
                    <TableHead>Permission</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rules.map((rule) => (
                    <TableRow key={rule.id}>
                      <TableCell className="font-medium">{getUsernameById(rule.user_id)}</TableCell>
                      <TableCell className="font-mono text-sm">{rule.topic_pattern}</TableCell>
                      <TableCell>{getPermissionBadge(rule.permission)}</TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => setDeleteRule(rule)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}

      {/* Help Card */}
      <Card>
        <CardHeader>
          <CardTitle>Topic Pattern Help</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>
            <strong>Wildcards:</strong>
          </p>
          <ul className="ml-4 space-y-1">
            <li>
              <code className="bg-muted rounded px-1">+</code> - Single level wildcard (e.g.,{' '}
              <code className="bg-muted rounded px-1">sensor/+/temperature</code>)
            </li>
            <li>
              <code className="bg-muted rounded px-1">#</code> - Multi-level wildcard (e.g.,{' '}
              <code className="bg-muted rounded px-1">device/#</code>)
            </li>
          </ul>
          <p className="pt-2">
            <strong>Permissions:</strong>
          </p>
          <ul className="ml-4 space-y-1">
            <li>
              <strong>pub</strong> - Publish only
            </li>
            <li>
              <strong>sub</strong> - Subscribe only
            </li>
            <li>
              <strong>pubsub</strong> - Both publish and subscribe
            </li>
          </ul>
        </CardContent>
      </Card>

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
              <div className="space-y-2">
                <Label htmlFor="user">User</Label>
                <Select value={userId.toString()} onValueChange={(v) => setUserId(parseInt(v))}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {users.map((user) => (
                      <SelectItem key={user.id} value={user.id.toString()}>
                        {user.username}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="topic">Topic Pattern</Label>
                <Input
                  id="topic"
                  placeholder="e.g., sensor/+/temp or device/#"
                  value={topicPattern}
                  onChange={(e) => setTopicPattern(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="permission">Permission</Label>
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
              </div>
              {error && <div className="text-destructive text-sm">{error}</div>}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? 'Creating...' : 'Create Rule'}
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
              Are you sure you want to delete this ACL rule? User{' '}
              <strong>{deleteRule && getUsernameById(deleteRule.user_id)}</strong> will lose access
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
