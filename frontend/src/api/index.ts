async function request<T>(path: string, options: RequestInit = {}): Promise<T | null> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  if (res.status === 401) {
    window.location.href = '/login'
    return null
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.message || 'Request failed')
  }
  if (res.status === 204) return null
  return res.json()
}

function get<T>(path: string) { return request<T>(path, { method: 'GET' }) }
function post<T>(path: string, body: unknown) { return request<T>(path, { method: 'POST', body: JSON.stringify(body) }) }
function put<T>(path: string, body: unknown) { return request<T>(path, { method: 'PUT', body: JSON.stringify(body) }) }
function del(path: string) { return request(path, { method: 'DELETE' }) }

export interface App {
  id: number
  name: string
  account_count?: number
}

export interface Account {
  id: number
  app_id: number
  identifier: string
  label?: string | null
  plugin_name?: string | null
}

export interface AccountDetail extends Account {
  has_credentials: boolean
}

export interface CredentialField {
  key: string
  label: string
  type: 'text' | 'password'
  required: boolean
  secret: boolean
}

export interface Plugin {
  id: number
  name: string
  version: string
  source: string
  description?: string | null
  status: string
}

export interface AccountPlugin {
  plugin_name: string | null
  version?: string
}

export interface BillItem {
  period: string
  amount: string
  due_date: string
  status: string
  invoice_number?: string
}

export interface BillFetchResponse {
  id: number
  account_id: number
  plugin_name: string
  plugin_version: string
  status: string
  bills: BillItem[]
}

export interface BillRecord {
  id: number
  account_id: number
  plugin_name: string
  plugin_version: string
  source: string
  status: string
  error?: string | null
  raw: string
  fetched_at: string
}

export interface BillsListResponse {
  items: BillRecord[]
  offset: number
}

export interface WebhookSettings {
  api_key: string
  enabled: boolean
}

export interface AccountWebhook {
  account_id: number
  webhook_url?: string | null
  schedule_time?: string | null
  schedule_enabled: boolean
  schedule_frequency: string
  schedule_interval?: number | null
  schedule_days?: string | null
  last_run_at?: string | null
}

export interface WebhookLog {
  id: number
  account_id: number
  webhook_url: string
  status: string
  http_status?: number | null
  payload: string
  response: string
  ran_at: string
}

export interface WebhookTestResult {
  delivered: number
  failed: number
}

export const api = {
  auth: {
    login: (email: string, password: string, remember: boolean) =>
      request('/api/login', { method: 'POST', body: JSON.stringify({ email, password, remember }) }),
    logout: () => fetch('/api/logout', { method: 'POST' }),
    setup: (email: string, password: string, password_confirm: string) =>
      request('/api/setup', { method: 'POST', body: JSON.stringify({ email, password, password_confirm }) }),
    setupStatus: () => get<{ setup: boolean }>('/api/setup-status'),
    me: () => get('/api/me'),
  },
  settings: {
    get: () => get<Record<string, string>>('/api/settings'),
    setLanguage: (language: string) => post('/api/settings/language', { language }),
  },
  webhooks: {
    getSettings: () => get<WebhookSettings>('/api/settings/webhook'),
    setSettings: (data: { api_key?: string; enabled?: boolean }) =>
      put<WebhookSettings>('/api/settings/webhook', data),
    account: (accountId: number) => get<AccountWebhook>(`/api/accounts/${accountId}/webhook`),
    setAccount: (accountId: number, data: {
      webhook_url?: string | null
      schedule_time?: string | null
      schedule_enabled?: boolean
      schedule_frequency?: string
      schedule_interval?: number | null
      schedule_days?: string | null
    }) => put<AccountWebhook>(`/api/accounts/${accountId}/webhook`, data),
    test: (accountId: number) => post<WebhookTestResult>(`/api/accounts/${accountId}/webhook:test`, {}),
    logs: (accountId: number, limit?: number, offset?: number) => {
      const params = new URLSearchParams()
      if (limit) params.set('limit', String(limit))
      if (offset) params.set('offset', String(offset))
      const qs = params.toString()
      return get<WebhookLog[]>(`/api/accounts/${accountId}/webhook/logs${qs ? `?${qs}` : ''}`)
    },
  },
  apps: {
    list: () => get<App[]>('/api/apps'),
    create: (name: string) => post<App>('/api/apps', { name }),
    update: (id: number, name: string) => put<App>(`/api/apps/${id}`, { name }),
    remove: (id: number) => del(`/api/apps/${id}`),
    accounts: (appId: number) => get<Account[]>(`/api/apps/${appId}/accounts`),
    account: (appId: number, accountId: number) => get<AccountDetail>(`/api/apps/${appId}/accounts/${accountId}`),
    createAccount: (appId: number, data: { identifier: string; label?: string | null; plugin_name?: string | null; credentials: object }) =>
      post<Account>(`/api/apps/${appId}/accounts`, data),
    updateAccount: (appId: number, accountId: number, data: { identifier: string; label?: string | null; credentials: object }) =>
      put<Account>(`/api/apps/${appId}/accounts/${accountId}`, data),
    removeAccount: (appId: number, accountId: number) => del(`/api/apps/${appId}/accounts/${accountId}`),
    revealCredentials: (accountId: number, password: string) =>
      post<{ credentials: string }>(`/api/accounts/${accountId}/credentials:reveal`, { password }),
  },
  plugins: {
    list: () => get<Plugin[]>('/api/plugins'),
    schema: (name: string) => get<CredentialField[]>(`/api/plugins/${encodeURIComponent(name)}/schema`),
  },
  bills: {
    accountPlugin: (accountId: number) => get<AccountPlugin>(`/api/accounts/${accountId}/plugin`),
    setAccountPlugin: (accountId: number, plugin_name: string | null) =>
      put<AccountPlugin>(`/api/accounts/${accountId}/plugin`, { plugin_name }),
    fetch: (accountId: number) => post<BillFetchResponse>(`/api/accounts/${accountId}/bills:fetch`, {}),
    list: (accountId: number, limit?: number, offset?: number, at?: string) => {
      const params = new URLSearchParams()
      if (limit) params.set('limit', String(limit))
      if (offset) params.set('offset', String(offset))
      if (at) params.set('at', at)
      const qs = params.toString()
      return get<BillsListResponse>(`/api/accounts/${accountId}/bills${qs ? `?${qs}` : ''}`)
    },
  },
}