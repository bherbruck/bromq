// API client with automatic JWT token handling

const API_BASE = '/api'

// DashboardUser - Dashboard users (login to web UI)
export interface DashboardUser {
  id: number
  username: string
  role: 'viewer' | 'admin'
  created_at: string
}

// MQTTUser - MQTT credentials for connecting to broker
export interface MQTTUser {
  id: number
  username: string
  description?: string
  metadata?: Record<string, any>
  provisioned_from_config: boolean
  created_at: string
  updated_at: string
}

// MQTTClient - Connected device tracking
export interface MQTTClient {
  id: number
  client_id: string
  mqtt_user_id?: number
  username?: string
  remote_addr: string
  connected_at: string
  disconnected_at?: string
  last_seen: string
  is_active: boolean
  metadata?: Record<string, any>
}

export interface ACLRule {
  id: number
  mqtt_user_id: number
  topic_pattern: string
  permission: 'pub' | 'sub' | 'pubsub'
  provisioned_from_config: boolean
}

export interface Client {
  id: string
  username: string
  remote: string
  listener: string
  protocol_version: number
  keepalive: number
  clean: boolean
  subscriptions_count: number
  inflight_count: number
  connected_at?: number // Unix timestamp from Prometheus
}

export interface ClientDetails {
  id: string
  username: string
  remote: string
  listener: string
  protocol_version: number
  keepalive: number
  clean: boolean
  subscriptions: SubscriptionInfo[]
  inflight_count: number
}

export interface SubscriptionInfo {
  topic: string
  qos: number
}

export interface Metrics {
  uptime: number
  connected_clients: number
  total_clients: number
  messages_received: number
  messages_sent: number
  messages_dropped: number
  packets_received: number
  packets_sent: number
  bytes_received: number
  bytes_sent: number
  subscriptions_total: number
  retained_messages: number
}

export interface PrometheusMetric {
  name: string
  labels: Record<string, string>
  value: number
}

export interface ClientMetrics {
  client_id: string
  messages_received: number
  messages_sent: number
  bytes_received: number
  bytes_sent: number
  packets_received: number
  packets_sent: number
}

export interface PaginationParams {
  page?: number
  pageSize?: number
  search?: string
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
}

export interface PaginationMetadata {
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface PaginatedResponse<T> {
  data: T[]
  pagination: PaginationMetadata
}

class APIClient {
  private getToken(): string | null {
    return localStorage.getItem('mqtt_token')
  }

  private setToken(token: string) {
    localStorage.setItem('mqtt_token', token)
  }

  removeToken() {
    localStorage.removeItem('mqtt_token')
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const token = this.getToken()
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    })

    if (!response.ok) {
      if (response.status === 401) {
        this.removeToken()
        throw new Error('Unauthorized')
      }
      const error = await response.json().catch(() => ({ error: response.statusText }))
      throw new Error(error.error || 'Request failed')
    }

    return response.json()
  }

  // Auth
  async login(username: string, password: string): Promise<{ token: string; user: DashboardUser }> {
    const result = await this.request<{ token: string; user: DashboardUser }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
    this.setToken(result.token)
    return result
  }

  async changePassword(currentPassword: string, newPassword: string): Promise<void> {
    return this.request<void>('/auth/change-password', {
      method: 'PUT',
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    })
  }

  // Dashboard Users (Users who log into the web interface)
  async getDashboardUsers(params?: PaginationParams): Promise<PaginatedResponse<DashboardUser>> {
    const queryString = this.buildQueryString(params)
    return this.request<PaginatedResponse<DashboardUser>>(`/dashboard/users${queryString}`)
  }

  async getDashboardUser(id: number): Promise<DashboardUser> {
    return this.request<DashboardUser>(`/dashboard/users/${id}`)
  }

  private buildQueryString(params?: PaginationParams): string {
    if (!params) return ''
    const query = new URLSearchParams()
    if (params.page) query.append('page', params.page.toString())
    if (params.pageSize) query.append('pageSize', params.pageSize.toString())
    if (params.search) query.append('search', params.search)
    if (params.sortBy) query.append('sortBy', params.sortBy)
    if (params.sortOrder) query.append('sortOrder', params.sortOrder)
    const str = query.toString()
    return str ? `?${str}` : ''
  }

  async createDashboardUser(username: string, password: string, role: 'viewer' | 'admin'): Promise<DashboardUser> {
    return this.request<DashboardUser>('/dashboard/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    })
  }

  async updateDashboardUser(id: number, username: string, role: 'viewer' | 'admin'): Promise<DashboardUser> {
    return this.request<DashboardUser>(`/dashboard/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ username, role }),
    })
  }

  async updateDashboardUserPassword(id: number, password: string): Promise<void> {
    return this.request<void>(`/dashboard/users/${id}/password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    })
  }

  async deleteDashboardUser(id: number): Promise<void> {
    return this.request<void>(`/dashboard/users/${id}`, {
      method: 'DELETE',
    })
  }

  // MQTT Users (Credentials for MQTT broker authentication)
  async getMQTTUsers(params?: PaginationParams): Promise<PaginatedResponse<MQTTUser>> {
    const queryString = this.buildQueryString(params)
    return this.request<PaginatedResponse<MQTTUser>>(`/mqtt/users${queryString}`)
  }

  async getMQTTUser(id: number): Promise<MQTTUser> {
    return this.request<MQTTUser>(`/mqtt/users/${id}`)
  }

  async createMQTTUser(username: string, password: string, description?: string, metadata?: Record<string, any>): Promise<MQTTUser> {
    return this.request<MQTTUser>('/mqtt/users', {
      method: 'POST',
      body: JSON.stringify({ username, password, description, metadata }),
    })
  }

  async updateMQTTUser(id: number, username: string, description?: string, metadata?: Record<string, any>): Promise<MQTTUser> {
    return this.request<MQTTUser>(`/mqtt/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ username, description, metadata }),
    })
  }

  async updateMQTTUserPassword(id: number, password: string): Promise<void> {
    return this.request<void>(`/mqtt/users/${id}/password`, {
      method: 'PUT',
      body: JSON.stringify({ password }),
    })
  }

  async deleteMQTTUser(id: number): Promise<void> {
    return this.request<void>(`/mqtt/users/${id}`, {
      method: 'DELETE',
    })
  }

  // MQTT Clients (Connected device tracking)
  async getMQTTClients(params?: PaginationParams & { activeOnly?: boolean }): Promise<PaginatedResponse<MQTTClient>> {
    const queryString = this.buildQueryString(params)
    const activeParam = params?.activeOnly ? 'active=true' : ''
    const separator = queryString && activeParam ? '&' : queryString ? '' : activeParam ? '?' : ''
    const fullQuery = queryString + (activeParam ? separator + activeParam : '')
    return this.request<PaginatedResponse<MQTTClient>>(`/mqtt/clients${fullQuery}`)
  }

  async getMQTTClientDetails(clientId: string): Promise<MQTTClient> {
    return this.request<MQTTClient>(`/mqtt/clients/${clientId}`)
  }

  async updateMQTTClientMetadata(clientId: string, metadata: Record<string, any>): Promise<void> {
    return this.request<void>(`/mqtt/clients/${clientId}/metadata`, {
      method: 'PUT',
      body: JSON.stringify({ metadata }),
    })
  }

  async deleteMQTTClient(id: number): Promise<void> {
    return this.request<void>(`/mqtt/clients/${id}`, {
      method: 'DELETE',
    })
  }

  // ACL
  async getACLRules(params?: PaginationParams): Promise<PaginatedResponse<ACLRule>> {
    const queryString = this.buildQueryString(params)
    return this.request<PaginatedResponse<ACLRule>>(`/acl${queryString}`)
  }

  async createACLRule(
    mqtt_user_id: number,
    topic_pattern: string,
    permission: 'pub' | 'sub' | 'pubsub'
  ): Promise<ACLRule> {
    return this.request<ACLRule>('/acl', {
      method: 'POST',
      body: JSON.stringify({ mqtt_user_id, topic_pattern, permission }),
    })
  }

  async updateACLRule(
    id: number,
    topic_pattern: string,
    permission: 'pub' | 'sub' | 'pubsub'
  ): Promise<ACLRule> {
    return this.request<ACLRule>(`/acl/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ topic_pattern, permission }),
    })
  }

  async deleteACLRule(id: number): Promise<void> {
    return this.request<void>(`/acl/${id}`, {
      method: 'DELETE',
    })
  }

  // Clients
  async getClients(): Promise<Client[]> {
    // Fetch both client list and Prometheus metrics
    const [clients, promMetrics] = await Promise.all([
      this.request<Client[]>('/clients'),
      this.getPrometheusMetrics().catch(() => []), // Don't fail if metrics unavailable
    ])

    // Enrich clients with connection time from Prometheus
    for (const client of clients) {
      const connectedMetric = promMetrics.find(
        (m) =>
          m.name === 'mqtt_client_connected_timestamp_seconds' && m.labels.client_id === client.id,
      )
      if (connectedMetric) {
        client.connected_at = connectedMetric.value
      }
    }

    return clients
  }

  async getClientDetails(id: string): Promise<ClientDetails> {
    return this.request<ClientDetails>(`/clients/${id}`)
  }

  async getPrometheusMetrics(): Promise<PrometheusMetric[]> {
    const response = await fetch('/metrics')
    if (!response.ok) {
      throw new Error('Failed to fetch metrics')
    }
    const text = await response.text()
    return this.parsePrometheusMetrics(text)
  }

  private parsePrometheusMetrics(text: string): PrometheusMetric[] {
    const metrics: PrometheusMetric[] = []
    const lines = text.split('\n')

    for (const line of lines) {
      // Skip comments and empty lines
      if (line.startsWith('#') || line.trim() === '') continue

      // Parse format: metric_name{label1="value1",label2="value2"} value
      const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)\{([^}]*)\}\s+([0-9.e+-]+)/)
      if (match) {
        const [, name, labelsStr, valueStr] = match
        const labels: Record<string, string> = {}

        // Parse labels
        const labelMatches = labelsStr.matchAll(/([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"/g)
        for (const labelMatch of labelMatches) {
          labels[labelMatch[1]] = labelMatch[2]
        }

        metrics.push({
          name,
          labels,
          value: parseFloat(valueStr),
        })
      }
    }

    return metrics
  }

  async getClientMetrics(id: string): Promise<ClientMetrics> {
    try {
      const allMetrics = await this.getPrometheusMetrics()

      // Filter metrics for this client
      const clientMetrics = allMetrics.filter((m) => m.labels.client_id === id)

      // Extract values
      const messagesReceived =
        clientMetrics.find((m) => m.name === 'mqtt_messages_received_total')?.value || 0
      const messagesSent =
        clientMetrics.find((m) => m.name === 'mqtt_messages_sent_total')?.value || 0
      const bytesReceived =
        clientMetrics.find((m) => m.name === 'mqtt_bytes_received_total')?.value || 0
      const bytesSent = clientMetrics.find((m) => m.name === 'mqtt_bytes_sent_total')?.value || 0
      const packetsReceived =
        clientMetrics.find((m) => m.name === 'mqtt_packets_received_total')?.value || 0
      const packetsSent =
        clientMetrics.find((m) => m.name === 'mqtt_packets_sent_total')?.value || 0

      return {
        client_id: id,
        messages_received: messagesReceived,
        messages_sent: messagesSent,
        bytes_received: bytesReceived,
        bytes_sent: bytesSent,
        packets_received: packetsReceived,
        packets_sent: packetsSent,
      }
    } catch (error) {
      console.error('Failed to fetch Prometheus metrics:', error)
      // Return empty metrics if fetch fails
      return {
        client_id: id,
        messages_received: 0,
        messages_sent: 0,
        bytes_received: 0,
        bytes_sent: 0,
        packets_received: 0,
        packets_sent: 0,
      }
    }
  }

  async disconnectClient(id: string): Promise<void> {
    return this.request<void>(`/clients/${id}/disconnect`, {
      method: 'POST',
    })
  }

  // Metrics
  async getMetrics(): Promise<Metrics> {
    return this.request<Metrics>('/metrics')
  }
}

export const api = new APIClient()
