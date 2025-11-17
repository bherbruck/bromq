import { ArrowLeft, Save, Trash2, X, Play, Plus, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import type { Route } from './+types/scripts.$id'
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
import { Alert, AlertDescription } from '~/components/ui/alert'
import { Badge } from '~/components/ui/badge'
import { Button } from '~/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import { Input } from '~/components/ui/input'
import { Field, FieldLabel } from '~/components/ui/field'
import { PageHeader } from '~/components/page-header'
import { PageTitle } from '~/components/page-title'
import { ScriptEditor } from '~/components/script-editor'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import { Switch } from '~/components/ui/switch'
import { Textarea } from '~/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '~/components/ui/dialog'
import { api, type UpdateScriptRequest, type CreateScriptTriggerRequest } from '~/lib/api'
import { toast } from 'sonner'

export const meta: Route.MetaFunction = ({ params }) => [
  { title: `Script #${params.id} - BroMQ` },
]

export default function ScriptDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()

  // State
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [isSaved, setIsSaved] = useState(true)
  const [error, setError] = useState('')

  // Form state
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [scriptContent, setScriptContent] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [triggers, setTriggers] = useState<CreateScriptTriggerRequest[]>([])

  // Logs state
  const [logPage, setLogPage] = useState(1)
  const [logLevel, setLogLevel] = useState<string>('')
  const pageSize = 50

  // Test dialog state
  const [testDialogOpen, setTestDialogOpen] = useState(false)
  const [testTriggerType, setTestTriggerType] = useState<string>('on_publish')
  const [testTopic, setTestTopic] = useState('test/topic')
  const [testPayload, setTestPayload] = useState('test payload')
  const [testClientId, setTestClientId] = useState('test-client')
  const [testResult, setTestResult] = useState<any>(null)
  const [isTesting, setIsTesting] = useState(false)

  // Fetch script
  const { data: script, isLoading } = useQuery({
    queryKey: ['scripts', id],
    queryFn: () => api.getScript(parseInt(id!)),
  })

  // Fetch logs
  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['script-logs', id, logPage, logLevel],
    queryFn: () => api.getScriptLogs(parseInt(id!), { page: logPage, page_size: pageSize, level: logLevel || undefined }),
  })

  // Initialize form when script loads
  useEffect(() => {
    if (script) {
      setName(script.name)
      setDescription(script.description || '')
      setScriptContent(script.content)
      setEnabled(script.enabled)
      setTriggers(
        script.triggers.map((t) => ({
          type: t.type,
          topic_filter: t.topic_filter,
          priority: t.priority,
          enabled: t.enabled,
        }))
      )
      setIsSaved(true)
    }
  }, [script])

  // Mark as unsaved when form changes
  const markUnsaved = () => setIsSaved(false)

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: (data: UpdateScriptRequest) => api.updateScript(parseInt(id!), data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scripts'] })
      queryClient.invalidateQueries({ queryKey: ['scripts', id] })
      toast.success('Script updated successfully')
      setIsSaved(true)
      setError('')
    },
    onError: (err: any) => {
      setError(err.message || 'Failed to update script')
      toast.error(err.message || 'Failed to update script')
    },
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: () => api.deleteScript(parseInt(id!)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scripts'] })
      toast.success('Script deleted successfully')
      navigate('/scripts')
    },
    onError: (err: any) => {
      toast.error(err.message || 'Failed to delete script')
    },
  })

  // Clear logs mutation
  const clearLogsMutation = useMutation({
    mutationFn: () => api.clearScriptLogs(parseInt(id!)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['script-logs', id] })
      toast.success('Logs cleared successfully')
      setLogPage(1)
    },
    onError: (err: any) => {
      toast.error(err.message || 'Failed to clear logs')
    },
  })

  // Test mutation
  const testMutation = useMutation({
    mutationFn: () =>
      api.testScript({
        content: scriptContent,
        type: testTriggerType,
        event_data: {
          topic: testTopic,
          payload: testPayload,
          clientId: testClientId,
          username: 'test-user',
        },
      }),
    onSuccess: (result) => {
      setTestResult(result)
      setIsTesting(false)
    },
    onError: (err: any) => {
      setTestResult({ success: false, error: err.message })
      setIsTesting(false)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!name.trim()) {
      setError('Script name is required')
      return
    }

    if (!scriptContent.trim()) {
      setError('Script content is required')
      return
    }

    if (triggers.length === 0) {
      setError('At least one trigger is required')
      return
    }

    updateMutation.mutate({
      name,
      description,
      content: scriptContent,
      enabled,
      triggers,
    })
  }

  const handleCancel = () => {
    if (script) {
      setName(script.name)
      setDescription(script.description || '')
      setScriptContent(script.content)
      setEnabled(script.enabled)
      setTriggers(
        script.triggers.map((t) => ({
          type: t.type,
          topic_filter: t.topic_filter,
          priority: t.priority,
          enabled: t.enabled,
        }))
      )
      setIsSaved(true)
      setError('')
    }
  }

  const addTrigger = () => {
    setTriggers([
      ...triggers,
      { type: 'on_publish', topic_filter: '', priority: 100, enabled: true },
    ])
    markUnsaved()
  }

  const removeTrigger = (index: number) => {
    setTriggers(triggers.filter((_, i) => i !== index))
    markUnsaved()
  }

  const updateTrigger = (index: number, updates: Partial<CreateScriptTriggerRequest>) => {
    setTriggers(triggers.map((t, i) => (i === index ? { ...t, ...updates } : t)))
    markUnsaved()
  }

  const handleTest = () => {
    setTestResult(null)
    setIsTesting(true)
    testMutation.mutate()
  }

  if (isLoading) {
    return (
      <div className="flex h-[calc(100vh-4rem)] items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!script) {
    return (
      <div className="container mx-auto py-6">
        <Alert variant="destructive">
          <AlertDescription>Script not found</AlertDescription>
        </Alert>
      </div>
    )
  }

  const isProvisioned = script.provisioned_from_config
  const canEdit = currentUser?.role === 'admin' && !isProvisioned
  const logs = logsData?.data || []
  const totalLogs = logsData?.pagination.total || 0
  const totalPages = logsData?.pagination.total_pages || 0

  return (
    <div className="space-y-6">
      <PageHeader
        title={
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" asChild>
              <Link to="/scripts">
                <ArrowLeft className="h-4 w-4" />
              </Link>
            </Button>
            <div>
              <PageTitle>{script.name}</PageTitle>
              {script.description && <p className="text-sm text-muted-foreground">{script.description}</p>}
            </div>
            {isProvisioned && (
              <Badge
                variant="secondary"
                className="bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300"
              >
                Provisioned
              </Badge>
            )}
            <Badge variant={enabled ? 'default' : 'secondary'}>
              {enabled ? 'Enabled' : 'Disabled'}
            </Badge>
          </div>
        }
        action={
          <div className="flex gap-2">
            {!isSaved && canEdit && (
              <>
                <Button variant="outline" onClick={handleCancel}>
                  <X className="mr-2 h-4 w-4" />
                  Cancel
                </Button>
                <Button onClick={handleSubmit} disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
                </Button>
              </>
            )}
            {isSaved && canEdit && (
              <>
                <Button variant="outline" onClick={() => setTestDialogOpen(true)}>
                  <Play className="mr-2 h-4 w-4" />
                  Test
                </Button>
                <Button variant="destructive" onClick={() => setIsDeleteDialogOpen(true)}>
                  <Trash2 className="mr-2 h-4 w-4" />
                  Delete
                </Button>
              </>
            )}
          </div>
        }
      />

      {isProvisioned && (
        <Alert className="mb-6">
          <AlertDescription>
            This script is provisioned from the configuration file and cannot be edited through the UI.
            To modify it, edit your config file and restart the server.
          </AlertDescription>
        </Alert>
      )}

      {error && (
        <div className="rounded-md bg-destructive/15 p-3 text-sm text-destructive mb-6">{error}</div>
      )}

      <div className="space-y-6">
          {/* Script Info */}
          <Card>
            <CardHeader>
              <CardTitle>Basic Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Field>
                <FieldLabel htmlFor="name">Name</FieldLabel>
                <Input
                  id="name"
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value)
                    markUnsaved()
                  }}
                  disabled={!canEdit}
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="description">Description</FieldLabel>
                <Textarea
                  id="description"
                  value={description}
                  onChange={(e) => {
                    setDescription(e.target.value)
                    markUnsaved()
                  }}
                  disabled={!canEdit}
                  rows={2}
                />
              </Field>

              <Field>
                <div className="flex items-center justify-between">
                  <FieldLabel htmlFor="enabled">Enabled</FieldLabel>
                  <Switch
                    id="enabled"
                    checked={enabled}
                    onCheckedChange={(checked) => {
                      setEnabled(checked)
                      markUnsaved()
                    }}
                    disabled={!canEdit}
                  />
                </div>
              </Field>
            </CardContent>
          </Card>

          {/* Script Code */}
          <Card>
            <CardHeader>
              <CardTitle>Script Code</CardTitle>
              <CardDescription>JavaScript code executed on triggers</CardDescription>
            </CardHeader>
            <CardContent>
              <ScriptEditor
                value={scriptContent}
                onChange={(val) => {
                  setScriptContent(val)
                  markUnsaved()
                }}
                readOnly={!canEdit}
                height="500px"
              />
              <p className="mt-2 text-sm text-muted-foreground">
                Available APIs: <code>event</code>, <code>log</code>, <code>mqtt</code>,{' '}
                <code>state</code>, <code>global</code>
              </p>
            </CardContent>
          </Card>

          {/* Triggers */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Triggers</CardTitle>
                  <CardDescription>Events that execute this script</CardDescription>
                </div>
                {canEdit && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={addTrigger}
                  >
                    <Plus className="mr-2 h-4 w-4" />
                    Add Trigger
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {triggers.map((trigger, index) => (
                <div key={index} className="rounded-lg border p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <h4 className="font-medium">Trigger {index + 1}</h4>
                    {canEdit && triggers.length > 1 && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        onClick={() => removeTrigger(index)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    )}
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <Field>
                      <FieldLabel>Trigger Type</FieldLabel>
                      {canEdit ? (
                        <Select
                          value={trigger.type}
                          onValueChange={(value) => updateTrigger(index, { type: value as any })}
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="on_publish">on_publish</SelectItem>
                            <SelectItem value="on_connect">on_connect</SelectItem>
                            <SelectItem value="on_disconnect">on_disconnect</SelectItem>
                            <SelectItem value="on_subscribe">on_subscribe</SelectItem>
                          </SelectContent>
                        </Select>
                      ) : (
                        <Badge variant="secondary">{trigger.type}</Badge>
                      )}
                    </Field>

                    <Field>
                      <FieldLabel>Priority</FieldLabel>
                      <Input
                        type="number"
                        value={trigger.priority}
                        onChange={(e) => updateTrigger(index, { priority: parseInt(e.target.value) })}
                        disabled={!canEdit}
                      />
                      {canEdit && <p className="text-xs text-muted-foreground">Lower = runs first</p>}
                    </Field>
                  </div>

                  {(trigger.type === 'on_publish' || trigger.type === 'on_subscribe') && (
                    <Field>
                      <FieldLabel>Topic Filter</FieldLabel>
                      <Input
                        value={trigger.topic_filter || ''}
                        onChange={(e) => updateTrigger(index, { topic_filter: e.target.value })}
                        placeholder="sensors/#"
                        disabled={!canEdit}
                      />
                      {canEdit && (
                        <p className="text-xs text-muted-foreground">
                          Use # (multi-level) or + (single-level) wildcards
                        </p>
                      )}
                    </Field>
                  )}

                  <Field>
                    <div className="flex items-center justify-between">
                      <FieldLabel>Enabled</FieldLabel>
                      <Switch
                        checked={trigger.enabled}
                        onCheckedChange={(checked) => updateTrigger(index, { enabled: checked })}
                        disabled={!canEdit}
                      />
                    </div>
                  </Field>
                </div>
              ))}
            </CardContent>
          </Card>

          {/* Logs */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Execution Logs</CardTitle>
                  <CardDescription>
                    {totalLogs} total log{totalLogs !== 1 ? 's' : ''}
                  </CardDescription>
                </div>
                <div className="flex gap-2">
                  <Select value={logLevel || 'all'} onValueChange={(val) => setLogLevel(val === 'all' ? '' : val)}>
                    <SelectTrigger className="w-[140px]">
                      <SelectValue placeholder="All levels" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">All levels</SelectItem>
                      <SelectItem value="debug">Debug</SelectItem>
                      <SelectItem value="info">Info</SelectItem>
                      <SelectItem value="warn">Warn</SelectItem>
                      <SelectItem value="error">Error</SelectItem>
                    </SelectContent>
                  </Select>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => queryClient.invalidateQueries({ queryKey: ['script-logs', id] })}
                    disabled={logsLoading}
                  >
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                  {canEdit && totalLogs > 0 && (
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={() => clearLogsMutation.mutate()}
                      disabled={clearLogsMutation.isPending}
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {logsLoading ? (
                <div className="flex justify-center py-8">
                  <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
                </div>
              ) : logs.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  No logs yet. Logs appear when the script executes.
                </div>
              ) : (
                <div className="space-y-2">
                  {logs.map((log) => (
                    <div
                      key={log.id}
                      className="rounded-md border bg-card p-3 text-sm"
                    >
                      <div className="flex items-start gap-3">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1 flex-wrap">
                            <Badge
                              variant={log.level === 'error' ? 'destructive' : 'secondary'}
                              className={
                                log.level === 'warn'
                                  ? 'bg-yellow-500/10 text-yellow-700 dark:text-yellow-400 border-yellow-500/20'
                                  : log.level === 'debug'
                                    ? 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20'
                                    : log.level === 'info'
                                      ? 'bg-green-500/10 text-green-700 dark:text-green-400 border-green-500/20'
                                      : ''
                              }
                            >
                              {log.level.toUpperCase()}
                            </Badge>
                            <Badge variant="outline" className="text-xs">
                              {log.type}
                            </Badge>
                            {log.execution_time_ms > 0 && (
                              <span className="text-xs text-muted-foreground">
                                {log.execution_time_ms}ms
                              </span>
                            )}
                            <span className="text-xs text-muted-foreground ml-auto">
                              {new Date(log.created_at).toLocaleString()}
                            </span>
                          </div>
                          <div className="break-words">{log.message}</div>
                          {log.context && (
                            <details className="mt-2">
                              <summary className="text-xs text-muted-foreground cursor-pointer hover:text-foreground">
                                Show context
                              </summary>
                              <pre className="mt-1 text-xs bg-muted p-2 rounded overflow-x-auto max-w-full">
                                {JSON.stringify(log.context, null, 2)}
                              </pre>
                            </details>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}

                  {/* Pagination */}
                  {totalPages > 1 && (
                    <div className="flex items-center justify-between pt-4">
                      <div className="text-sm text-muted-foreground">
                        Page {logPage} of {totalPages}
                      </div>
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setLogPage((p) => Math.max(1, p - 1))}
                          disabled={logPage === 1}
                        >
                          Previous
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setLogPage((p) => Math.min(totalPages, p + 1))}
                          disabled={logPage === totalPages}
                        >
                          Next
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
      </div>

      {/* Delete confirmation dialog */}
      <AlertDialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Script</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete "{script.name}"? This will also delete all logs and state
              for this script. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteMutation.mutate()}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Test Script Dialog */}
      <Dialog open={testDialogOpen} onOpenChange={setTestDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Test Script</DialogTitle>
            <DialogDescription>Test with mock event data</DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <Field>
              <FieldLabel>Trigger Type</FieldLabel>
              <Select value={testTriggerType} onValueChange={setTestTriggerType}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="on_publish">on_publish</SelectItem>
                  <SelectItem value="on_connect">on_connect</SelectItem>
                  <SelectItem value="on_disconnect">on_disconnect</SelectItem>
                  <SelectItem value="on_subscribe">on_subscribe</SelectItem>
                </SelectContent>
              </Select>
            </Field>

            {testTriggerType === 'on_publish' && (
              <>
                <Field>
                  <FieldLabel>Topic</FieldLabel>
                  <Input value={testTopic} onChange={(e) => setTestTopic(e.target.value)} />
                </Field>
                <Field>
                  <FieldLabel>Payload</FieldLabel>
                  <Input value={testPayload} onChange={(e) => setTestPayload(e.target.value)} />
                </Field>
              </>
            )}

            <Field>
              <FieldLabel>Client ID</FieldLabel>
              <Input value={testClientId} onChange={(e) => setTestClientId(e.target.value)} />
            </Field>

            <Button onClick={handleTest} disabled={isTesting} className="w-full">
              {isTesting ? 'Running...' : 'Run Test'}
            </Button>

            {testResult && (
              <div className="rounded-md border p-4 space-y-2">
                <div className="flex items-center gap-2">
                  <strong>Result:</strong>
                  {testResult.success ? (
                    <Badge className="bg-green-600">Success</Badge>
                  ) : (
                    <Badge variant="destructive">Error</Badge>
                  )}
                  {testResult.execution_time_ms !== undefined && (
                    <span className="text-sm text-muted-foreground">
                      {testResult.execution_time_ms}ms
                    </span>
                  )}
                </div>

                {testResult.error && (
                  <div className="text-sm text-destructive bg-destructive/10 p-2 rounded">
                    {testResult.error}
                  </div>
                )}

                {testResult.logs && testResult.logs.length > 0 && (
                  <div className="space-y-1">
                    <strong className="text-sm">Logs:</strong>
                    {testResult.logs.map((log: any, idx: number) => (
                      <div
                        key={idx}
                        className={`text-sm font-mono p-2 rounded ${
                          log.level === 'error'
                            ? 'bg-red-900/20 text-red-400'
                            : log.level === 'warn'
                              ? 'bg-yellow-900/20 text-yellow-400'
                              : 'bg-gray-900/20'
                        }`}
                      >
                        [{log.level.toUpperCase()}] {log.message}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
