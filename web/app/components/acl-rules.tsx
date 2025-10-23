import { HelpCircle, Pencil, Plus, Shield, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useAuth } from '~/lib/auth-context'
import { HoverCard, HoverCardContent, HoverCardTrigger } from '~/components/ui/hover-card'
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
import { Field, FieldLabel, FieldError } from '~/components/ui/field'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '~/components/ui/empty'
import { Spinner } from '~/components/ui/spinner'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '~/components/ui/table'
import { api, type ACLRule, type MQTTUser } from '~/lib/api'

interface ACLRulesProps {
  mqttUserId?: number // Optional: filter rules by MQTT user ID
  showHeader?: boolean // Show card header (default: true)
  showHelp?: boolean // Show help card (default: true)
  showMQTTUserColumn?: boolean // Show MQTT user column in table (default: true)
}

export function ACLRules({
  mqttUserId,
  showHeader = true,
  showHelp = true,
  showMQTTUserColumn = true,
}: ACLRulesProps) {
  const { user: currentUser } = useAuth()
  const [rules, setRules] = useState<ACLRule[]>([])
  const [mqttUsers, setMqttUsers] = useState<MQTTUser[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<ACLRule | null>(null)
  const [deleteRule, setDeleteRule] = useState<ACLRule | null>(null)

  // Form state
  const [selectedMqttUserId, setSelectedMqttUserId] = useState<number>(mqttUserId || 0)
  const [topicPattern, setTopicPattern] = useState('')
  const [permission, setPermission] = useState<'pub' | 'sub' | 'pubsub'>('pubsub')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState('')

  const fetchData = async () => {
    try {
      const [rulesData, usersData] = await Promise.all([api.getACLRules(), api.getMQTTUsers()])

      // Filter rules by MQTT user if specified
      const filteredRules = mqttUserId
        ? rulesData.filter((rule) => rule.mqtt_user_id === mqttUserId)
        : rulesData

      setRules(filteredRules)
      setMqttUsers(usersData)

      // Set default selected user ID if not set
      if (!mqttUserId && usersData.length > 0 && selectedMqttUserId === 0) {
        setSelectedMqttUserId(usersData[0].id)
      }
    } catch (error) {
      console.error('Failed to fetch data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [mqttUserId])

  // Update selectedMqttUserId when mqttUserId prop changes
  useEffect(() => {
    if (mqttUserId) {
      setSelectedMqttUserId(mqttUserId)
    }
  }, [mqttUserId])

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

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsSubmitting(true)

    try {
      await api.createACLRule(selectedMqttUserId, topicPattern, permission)
      setIsCreateDialogOpen(false)
      resetForm()
      fetchData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create ACL rule')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleEditClick = (rule: ACLRule) => {
    setEditingRule(rule)
    setTopicPattern(rule.topic_pattern)
    setPermission(rule.permission as 'pub' | 'sub' | 'pubsub')
    setIsEditDialogOpen(true)
  }

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingRule) return

    setError('')
    setIsSubmitting(true)

    try {
      await api.updateACLRule(editingRule.id, topicPattern, permission)
      setIsEditDialogOpen(false)
      setEditingRule(null)
      resetForm()
      fetchData()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update ACL rule')
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

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Spinner />
        Loading ACL rules...
      </div>
    )
  }

  const canEdit = currentUser?.role === 'admin'

  return (
    <div className="space-y-6">
      {canEdit && (
        <div className="flex items-center justify-end">
          <Button onClick={() => setIsCreateDialogOpen(true)} disabled={mqttUsers.length === 0}>
            <Plus className="mr-2 h-4 w-4" />
            Add Rule
          </Button>
        </div>
      )}

      {mqttUsers.length === 0 ? (
        <Card>
          <CardContent className="py-8">
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
          </CardContent>
        </Card>
      ) : (
        <Card>
          {showHeader && (
            <CardHeader>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <CardTitle>Access Control Rules</CardTitle>
                  {showHelp && (
                    <HoverCard>
                      <HoverCardTrigger asChild>
                        <button className="text-muted-foreground hover:text-foreground">
                          <HelpCircle className="h-4 w-4" />
                        </button>
                      </HoverCardTrigger>
                      <HoverCardContent className="w-80">
                        <div className="space-y-2 text-sm">
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
                        </div>
                      </HoverCardContent>
                    </HoverCard>
                  )}
                </div>
              </div>
              <CardDescription>
                {rules.length} rule{rules.length !== 1 ? 's' : ''} configured
                {mqttUserId && ` for this user`}
              </CardDescription>
            </CardHeader>
          )}
          <CardContent className={showHeader ? '' : 'pt-6'}>
            {rules.length === 0 ? (
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
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    {showMQTTUserColumn && <TableHead>MQTT User</TableHead>}
                    <TableHead>Topic Pattern</TableHead>
                    <TableHead>Permission</TableHead>
                    {canEdit && <TableHead className="text-right">Actions</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rules.map((rule) => (
                    <TableRow key={rule.id}>
                      {showMQTTUserColumn && (
                        <TableCell className="font-medium">{getMQTTUsernameById(rule.mqtt_user_id)}</TableCell>
                      )}
                      <TableCell className="font-mono text-sm">{rule.topic_pattern}</TableCell>
                      <TableCell>{getPermissionBadge(rule.permission)}</TableCell>
                      {canEdit && (
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => handleEditClick(rule)}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="destructive"
                              size="sm"
                              onClick={() => setDeleteRule(rule)}
                            >
                              <Trash2 className="h-4 w-4" />
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
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Spinner className="mr-2" />}
                {isSubmitting ? 'Creating...' : 'Create Rule'}
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
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting && <Spinner className="mr-2" />}
                {isSubmitting ? 'Updating...' : 'Update Rule'}
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
