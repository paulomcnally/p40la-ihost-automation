import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { useToast } from '../components/Toast'
import { Icon } from '../components/Icons'
import Dropdown, { type DropdownOption } from '../components/Dropdown'
import { api, type Account, type App, type CredentialField, type Plugin } from '../api'

interface AccountModalProps {
  appId: number
  initial?: Account | null
  onClose: () => void
  onSaved: () => void
}

function AccountModal({ appId, initial, onClose, onSaved }: AccountModalProps) {
  const { t } = useI18nStore()
  const { showToast } = useToast()
  const [identifier, setIdentifier] = useState(initial?.identifier ?? '')
  const [label, setLabel] = useState(initial?.label ?? '')
  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [selectedPlugin, setSelectedPlugin] = useState('')
  const [schema, setSchema] = useState<CredentialField[] | null>(null)
  const [schemaLoading, setSchemaLoading] = useState(false)
  const [values, setValues] = useState<Record<string, string>>({})
  const [hasCredentials, setHasCredentials] = useState(false)
  const [revealed, setRevealed] = useState(false)
  const [showRevealDialog, setShowRevealDialog] = useState(false)
  const [revealPassword, setRevealPassword] = useState('')
  const [revealing, setRevealing] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (initial) return
    api.plugins.list()
      .then((list) => setPlugins(list ?? []))
      .catch(() => {})
  }, [initial])

  useEffect(() => {
    const pluginName = initial?.plugin_name ?? (selectedPlugin || null)
    setHasCredentials(false)
    setSchema(null)
    if (!pluginName) return
    setSchemaLoading(true)
    api.plugins.schema(pluginName)
      .then((s) => setSchema(s ?? []))
      .catch(() => setSchema(null))
      .finally(() => setSchemaLoading(false))
    if (initial) {
      api.apps.account(appId, initial.id)
        .then((detail) => setHasCredentials(detail?.has_credentials ?? false))
        .catch(() => {})
    }
  }, [initial, selectedPlugin])

  const buildCredentials = (): object | null => {
    if (!initial && !selectedPlugin) return {}
    if (schema) {
      const obj: Record<string, string> = {}
      for (const field of schema) {
        const v = (values[field.key] ?? '').trim()
        if (field.required && !v) return null
        if (v) obj[field.key] = v
      }
      return obj
    }
    // Fallback: textarea JSON libre.
    try {
      const parsed = JSON.parse(values.json_raw ?? '')
      if (typeof parsed !== 'object' || Array.isArray(parsed)) return null
      return parsed
    } catch {
      return null
    }
  }

  const handlePluginChange = (name: string) => {
    setSelectedPlugin(name)
    setValues({})
    setSchema(null)
    setError('')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const credentials = buildCredentials()
    if (!credentials) {
      setError(t('apps.invalid_json'))
      return
    }

    const data = {
      identifier,
      label: label.trim() ? label.trim() : null,
      plugin_name: initial ? undefined : (selectedPlugin || null),
      credentials,
    }

    setLoading(true)
    try {
      if (initial) {
        await api.apps.updateAccount(appId, initial.id, data)
      } else {
        await api.apps.createAccount(appId, data)
      }
      showToast(t('apps.saved'), 'success')
      onSaved()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('errors.generic'))
    } finally {
      setLoading(false)
    }
  }

  const handleReveal = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!initial || !revealPassword) return
    setRevealing(true)
    setError('')
    try {
      const result = await api.apps.revealCredentials(initial.id, revealPassword)
      if (!result) return
      const parsed = JSON.parse(result.credentials) as Record<string, string>
      const next = { ...values }
      for (const [k, v] of Object.entries(parsed)) {
        if (typeof v === 'string') next[k] = v
      }
      setValues(next)
      setRevealed(true)
      setShowRevealDialog(false)
      setRevealPassword('')
    } catch (err: unknown) {
      setError(t('apps.credentials_reveal_error'))
    } finally {
      setRevealing(false)
    }
  }

  const renderCredentialInput = (field: CredentialField) => {
    const isSecret = field.secret
    const isRevealed = revealed && isSecret
    const current = values[field.key] ?? ''
    return (
      <div key={field.key}>
        <label className="block text-sm font-medium mb-1">
          {field.label} {field.required ? '*' : `(${t('apps.credentials_optional')})`}
        </label>
        <div className="flex gap-2">
          <input
            type={isSecret && !isRevealed ? 'password' : 'text'}
            value={current}
            onChange={(e) => setValues({ ...values, [field.key]: e.target.value })}
            placeholder={isSecret ? '••••••••' : field.label}
            required={field.required}
            className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
          />
          {isSecret && initial && hasCredentials && !isRevealed && (
            <button
              type="button"
              onClick={() => setShowRevealDialog(true)}
              className="px-3 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg transition-colors min-h-[44px] flex-shrink-0"
              title={t('apps.credentials_show')}
            >
              <Icon name="eye" className="w-4 h-4" />
            </button>
          )}
          {isSecret && isRevealed && (
            <button
              type="button"
              onClick={() => {
                setRevealed(false)
                setValues({ ...values, [field.key]: '' })
              }}
              className="px-3 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg transition-colors min-h-[44px] flex-shrink-0"
              title={t('apps.credentials_hide')}
            >
              <Icon name="eye-off" className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 bg-black/30 z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div className="bg-card rounded-ios shadow-ios p-5 w-full max-w-md max-h-[90vh] overflow-y-auto" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-semibold mb-4">{initial ? t('apps.edit_account') : t('apps.create_account')}</h3>
        {error && (
          <div className="bg-danger/10 text-danger text-sm p-3 rounded-ios-sm mb-4">{error}</div>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          {!initial && (
            <div>
              <label className="block text-sm font-medium mb-1">{t('apps.plugin')}</label>
              <Dropdown
                value={selectedPlugin}
                onChange={handlePluginChange}
                placeholder={t('apps.plugin_none')}
                options={[
                  { value: '', label: t('apps.plugin_none') },
                  ...plugins.map((p): DropdownOption => ({ value: p.name, label: p.name })),
                ]}
              />
            </div>
          )}
          <div>
            <label className="block text-sm font-medium mb-1">{t('apps.identifier')} *</label>
            <input
              type="text"
              value={identifier}
              onChange={(e) => setIdentifier(e.target.value)}
              className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              placeholder={t('apps.identifier_placeholder')}
              required
              autoFocus
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">{t('apps.label')}</label>
            <input
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              placeholder={t('apps.label_placeholder')}
            />
          </div>

          {!initial && !selectedPlugin ? (
            <div>
              <label className="block text-sm font-medium mb-1">{t('apps.credentials')}</label>
              <p className="text-sm text-text-secondary">{t('apps.credentials_plugin_hint')}</p>
            </div>
          ) : schemaLoading ? (
            <div className="text-text-secondary text-sm py-4 text-center">{t('app.loading')}</div>
          ) : schema && schema.length > 0 ? (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <label className="block text-sm font-medium">{t('apps.credentials')} *</label>
                {initial && hasCredentials && !revealed && (
                  <button
                    type="button"
                    onClick={() => setShowRevealDialog(true)}
                    className="flex items-center gap-1 text-sm text-primary hover:opacity-80 transition-opacity min-h-[44px]"
                  >
                    <Icon name="eye" className="w-4 h-4" />
                    {t('apps.credentials_show')}
                  </button>
                )}
              </div>
              {schema.map(renderCredentialInput)}
            </div>
          ) : (
            <div>
              <label className="block text-sm font-medium mb-1">{t('apps.credentials')} *</label>
              <textarea
                value={values.json_raw ?? ''}
                onChange={(e) => setValues({ ...values, json_raw: e.target.value })}
                rows={6}
                className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary font-mono text-sm min-h-[44px]"
                placeholder={t('apps.credentials_placeholder')}
                required
                disabled={schemaLoading}
              />
              <p className="text-xs text-text-secondary mt-1">{t('apps.credentials_hint')}</p>
            </div>
          )}

          <div className="flex gap-2 justify-end">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg transition-colors min-h-[44px]"
            >
              {t('app.cancel')}
            </button>
            <button
              type="submit"
              disabled={loading || schemaLoading}
              className="px-4 py-2 rounded-ios-sm bg-primary text-white font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
            >
              {loading ? '...' : t('app.save')}
            </button>
          </div>
        </form>
      </div>

      {showRevealDialog && (
        <div className="fixed inset-0 bg-black/30 z-60 flex items-center justify-center p-4" onClick={() => setShowRevealDialog(false)}>
          <div className="bg-card rounded-ios shadow-ios p-5 w-full max-w-sm" onClick={(e) => e.stopPropagation()}>
            <h4 className="text-base font-semibold mb-1">{t('apps.credentials_reveal_title')}</h4>
            <p className="text-sm text-text-secondary mb-4">{t('apps.credentials_reveal_hint')}</p>
            <form onSubmit={handleReveal} className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-1">{t('apps.credentials_password_label')}</label>
                <input
                  type="password"
                  value={revealPassword}
                  onChange={(e) => setRevealPassword(e.target.value)}
                  className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
                  autoFocus
                />
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setShowRevealDialog(false)}
                  className="px-4 py-2 rounded-ios-sm border border-border text-text-secondary hover:bg-bg transition-colors min-h-[44px]"
                >
                  {t('app.cancel')}
                </button>
                <button
                  type="submit"
                  disabled={revealing || !revealPassword}
                  className="px-4 py-2 rounded-ios-sm bg-primary text-white font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
                >
                  {revealing ? '...' : t('apps.credentials_show')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

function maskedCredentials(): string {
  return '••••••••••••'
}

export default function AppDetailPage() {
  const { appId } = useParams()
  const id = Number(appId)
  const navigate = useNavigate()
  const { t } = useI18nStore()
  const { showToast } = useToast()
  usePageTitle(t('apps.accounts'))

  const [app, setApp] = useState<App | null>(null)
  const [accounts, setAccounts] = useState<Account[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Account | null>(null)

  const load = async () => {
    if (!Number.isFinite(id)) return
    setLoading(true)
    try {
      const apps = await api.apps.list()
      const current = (apps ?? []).find((a) => a.id === id)
      setApp(current ?? null)
      const data = await api.apps.accounts(id)
      setAccounts(data ?? [])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [id])

  const handleDelete = async (account: Account) => {
    if (!window.confirm(t('apps.delete_account_confirm'))) return
    try {
      await api.apps.removeAccount(id, account.id)
      showToast(t('apps.deleted'), 'success')
      load()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    }
  }

  const handleSaved = () => {
    setModalOpen(false)
    setEditing(null)
    load()
  }

  return (
    <div className="max-w-2xl mx-auto">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-xl sm:text-2xl font-bold truncate">{app?.name ?? t('apps.accounts')}</h2>
        <button
          onClick={() => { setEditing(null); setModalOpen(true) }}
          className="flex items-center gap-2 bg-primary text-white px-4 py-2 rounded-ios-sm font-medium hover:bg-primary-hover transition-colors min-h-[44px] flex-shrink-0"
        >
          <Icon name="plus" className="w-4 h-4" />
          {t('apps.create_account')}
        </button>
      </div>
      <p className="text-text-secondary mb-6">{t('apps.accounts_subtitle')}</p>

      {loading ? (
        <div className="text-text-secondary py-10 text-center">{t('app.loading')}</div>
      ) : accounts.length === 0 ? (
        <div className="bg-card rounded-ios shadow-ios p-10 flex flex-col items-center justify-center text-center">
          <Icon name="key" className="w-12 h-12 text-text-secondary/40 mb-3" />
          <p className="text-text-secondary">{t('apps.accounts_empty')}</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {accounts.map((account) => (
            <li key={account.id}>
              <div className="bg-card rounded-ios shadow-ios p-4 flex items-center justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Icon name="key" className="w-4 h-4 text-primary flex-shrink-0" />
                    <span className="font-medium truncate">{account.identifier}</span>
                    {account.label && (
                      <span className="text-sm text-text-secondary bg-bg px-2 py-0.5 rounded-ios-sm truncate">
                        {account.label}
                      </span>
                    )}
                  </div>
                  <div className="mt-1 text-sm text-text-secondary flex items-center gap-1">
                    <span className="font-mono">{maskedCredentials()}</span>
                    {account.plugin_name && (
                      <span className="ml-1 text-xs bg-primary/10 text-primary px-2 py-0.5 rounded-ios-sm">
                        {account.plugin_name}
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-1 flex-shrink-0">
                  <button
                    onClick={() => navigate(`/apps/${id}/accounts/${account.id}/bills`)}
                    className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
                    title={t('bills.view')}
                  >
                    <Icon name="bill" className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => { setEditing(account); setModalOpen(true) }}
                    className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
                    title={t('apps.edit_account')}
                  >
                    <Icon name="edit" className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(account)}
                    className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
                    title={t('apps.delete_account')}
                  >
                    <Icon name="trash" className="w-4 h-4 text-danger" />
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {modalOpen && (
        <AccountModal
          appId={id}
          initial={editing}
          onClose={() => { setModalOpen(false); setEditing(null) }}
          onSaved={handleSaved}
        />
      )}
    </div>
  )
}