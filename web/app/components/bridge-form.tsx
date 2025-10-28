import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button } from '~/components/ui/button'
import { Input } from '~/components/ui/input'
import { Field, FieldLabel, FieldError } from '~/components/ui/field'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card'
import type { Bridge, BridgeTopicRequest, CreateBridgeRequest, UpdateBridgeRequest } from '~/lib/api'

interface BridgeFormProps {
  mode: 'create' | 'edit'
  initialData?: Bridge
  onSubmit: (data: CreateBridgeRequest | UpdateBridgeRequest) => void
  isSubmitting: boolean
  error: string
}

export function BridgeForm({ mode, initialData, onSubmit, isSubmitting, error }: BridgeFormProps) {
  // Connection settings
  const [name, setName] = useState(initialData?.name || '')
  const [remoteHost, setRemoteHost] = useState(initialData?.remote_host || '')
  const [remotePort, setRemotePort] = useState(initialData?.remote_port?.toString() || '1883')
  const [remoteUsername, setRemoteUsername] = useState(initialData?.remote_username || '')
  const [remotePassword, setRemotePassword] = useState('')
  const [clientId, setClientId] = useState(initialData?.client_id || '')
  const [cleanSession, setCleanSession] = useState(initialData?.clean_session ?? true)
  const [keepAlive, setKeepAlive] = useState(initialData?.keep_alive?.toString() || '60')
  const [connectionTimeout, setConnectionTimeout] = useState(initialData?.connection_timeout?.toString() || '30')

  // Topic mappings
  const [topics, setTopics] = useState<BridgeTopicRequest[]>(
    initialData?.topics?.map(t => ({
      local_pattern: t.local_pattern,
      remote_pattern: t.remote_pattern,
      direction: t.direction,
      qos: t.qos,
    })) || []
  )

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    const data: CreateBridgeRequest | UpdateBridgeRequest = {
      name,
      remote_host: remoteHost,
      remote_port: parseInt(remotePort, 10),
      remote_username: remoteUsername || undefined,
      remote_password: remotePassword || undefined,
      client_id: clientId || undefined,
      clean_session: cleanSession,
      keep_alive: parseInt(keepAlive, 10),
      connection_timeout: parseInt(connectionTimeout, 10),
      topics,
    }

    onSubmit(data)
  }

  const addTopic = () => {
    setTopics([...topics, { local_pattern: '', remote_pattern: '', direction: 'both', qos: 0 }])
  }

  const removeTopic = (index: number) => {
    setTopics(topics.filter((_, i) => i !== index))
  }

  const updateTopic = (index: number, field: keyof BridgeTopicRequest, value: string | number) => {
    const newTopics = [...topics]
    newTopics[index] = { ...newTopics[index], [field]: value }
    setTopics(newTopics)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Connection Settings */}
      <Card>
        <CardHeader>
          <CardTitle>Connection Settings</CardTitle>
          <CardDescription>Configure the remote MQTT broker connection</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Field>
            <FieldLabel htmlFor="name">Bridge Name *</FieldLabel>
            <Input
              id="name"
              placeholder="my-bridge"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </Field>

          <div className="grid grid-cols-2 gap-4">
            <Field>
              <FieldLabel htmlFor="remote-host">Remote Host *</FieldLabel>
              <Input
                id="remote-host"
                placeholder="broker.example.com"
                value={remoteHost}
                onChange={(e) => setRemoteHost(e.target.value)}
                required
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="remote-port">Remote Port *</FieldLabel>
              <Input
                id="remote-port"
                type="number"
                placeholder="1883"
                value={remotePort}
                onChange={(e) => setRemotePort(e.target.value)}
                required
              />
            </Field>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <Field>
              <FieldLabel htmlFor="remote-username">Username</FieldLabel>
              <Input
                id="remote-username"
                placeholder="Optional"
                value={remoteUsername}
                onChange={(e) => setRemoteUsername(e.target.value)}
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="remote-password">Password</FieldLabel>
              <Input
                id="remote-password"
                type="password"
                placeholder={mode === 'edit' ? 'Leave blank to keep current' : 'Optional'}
                value={remotePassword}
                onChange={(e) => setRemotePassword(e.target.value)}
              />
            </Field>
          </div>

          <Field>
            <FieldLabel htmlFor="client-id">Client ID</FieldLabel>
            <Input
              id="client-id"
              placeholder="Auto-generated if empty"
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
            />
          </Field>

          <div className="grid grid-cols-3 gap-4">
            <Field>
              <FieldLabel htmlFor="keep-alive">Keep Alive (s) *</FieldLabel>
              <Input
                id="keep-alive"
                type="number"
                value={keepAlive}
                onChange={(e) => setKeepAlive(e.target.value)}
                required
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="connection-timeout">Timeout (s) *</FieldLabel>
              <Input
                id="connection-timeout"
                type="number"
                value={connectionTimeout}
                onChange={(e) => setConnectionTimeout(e.target.value)}
                required
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="clean-session">Clean Session *</FieldLabel>
              <Select value={cleanSession.toString()} onValueChange={(v) => setCleanSession(v === 'true')}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="true">Yes</SelectItem>
                  <SelectItem value="false">No</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </div>
        </CardContent>
      </Card>

      {/* Topic Mappings */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Topic Mappings</CardTitle>
              <CardDescription>Define topic routing rules between brokers</CardDescription>
            </div>
            <Button type="button" variant="outline" size="sm" onClick={addTopic}>
              <Plus className="mr-2 h-4 w-4" />
              Add Topic
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {topics.length === 0 ? (
            <p className="text-muted-foreground text-center text-sm">
              No topic mappings. Click "Add Topic" to create one.
            </p>
          ) : (
            topics.map((topic, index) => (
              <div key={index} className="bg-muted/50 space-y-4 rounded-lg border p-4">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">Topic Mapping {index + 1}</p>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeTopic(index)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <Field>
                    <FieldLabel>Local Pattern *</FieldLabel>
                    <Input
                      placeholder="local/topic/#"
                      value={topic.local_pattern}
                      onChange={(e) => updateTopic(index, 'local_pattern', e.target.value)}
                      required
                    />
                  </Field>

                  <Field>
                    <FieldLabel>Remote Pattern *</FieldLabel>
                    <Input
                      placeholder="remote/topic/#"
                      value={topic.remote_pattern}
                      onChange={(e) => updateTopic(index, 'remote_pattern', e.target.value)}
                      required
                    />
                  </Field>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <Field>
                    <FieldLabel>Direction *</FieldLabel>
                    <Select
                      value={topic.direction}
                      onValueChange={(v) => updateTopic(index, 'direction', v)}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="in">Inbound (Remote → Local)</SelectItem>
                        <SelectItem value="out">Outbound (Local → Remote)</SelectItem>
                        <SelectItem value="both">Bidirectional</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>

                  <Field>
                    <FieldLabel>QoS *</FieldLabel>
                    <Select
                      value={topic.qos.toString()}
                      onValueChange={(v) => updateTopic(index, 'qos', parseInt(v, 10))}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="0">QoS 0 (At most once)</SelectItem>
                        <SelectItem value="1">QoS 1 (At least once)</SelectItem>
                        <SelectItem value="2">QoS 2 (Exactly once)</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      {error && <FieldError>{error}</FieldError>}

      <div className="flex justify-end gap-2">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Saving...' : mode === 'create' ? 'Create Bridge' : 'Update Bridge'}
        </Button>
      </div>
    </form>
  )
}
