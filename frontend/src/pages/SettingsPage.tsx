import { useEffect, useState } from 'react'
import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { useToast } from '../components/Toast'
import { api, type WebhookSettings } from '../api'

function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      className={`w-12 h-7 rounded-full relative transition-colors flex-shrink-0 ${checked ? 'bg-primary' : 'bg-border'}`}
    >
      <span
        className={`absolute top-1 w-5 h-5 rounded-full bg-white transition-all ${checked ? 'left-6' : 'left-1'}`}
      />
    </button>
  )
}

export default function SettingsPage() {
  const { t } = useI18nStore()
  const { showToast } = useToast()
  usePageTitle(t('settings.title'))

  const [config, setConfig] = useState<WebhookSettings | null>(null)
  const [apiKey, setApiKey] = useState('')
  const [enabled, setEnabled] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const data = await api.webhooks.getSettings()
      setConfig(data ?? { api_key: '', enabled: false })
      setApiKey(data?.api_key ?? '')
      setEnabled(data?.enabled ?? false)
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const handleSave = async () => {
    setError('')
    setSaving(true)
    try {
      await api.webhooks.setSettings({ api_key: apiKey, enabled })
      showToast(t('settings.saved'), 'success')
      load()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('errors.generic'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-xl sm:text-2xl font-bold mb-2">{t('settings.title')}</h2>
      <p className="text-text-secondary mb-6">{t('settings.subtitle')}</p>

      <div className="bg-card rounded-ios shadow-ios p-5">
        <div className="flex items-center justify-between gap-3 mb-1">
          <div>
            <h3 className="font-semibold">{t('settings.webhooks.title')}</h3>
            <p className="text-sm text-text-secondary">{t('settings.webhooks.subtitle')}</p>
          </div>
          <Toggle checked={enabled} onChange={setEnabled} />
        </div>

        {error && (
          <div className="bg-danger/10 text-danger text-sm p-3 rounded-ios-sm mt-3">{error}</div>
        )}

        {loading ? (
          <div className="text-text-secondary py-6 text-center">{t('app.loading')}</div>
        ) : (
          <div className="mt-4 space-y-4">
            <div>
              <label className="block text-sm font-medium mb-1">{t('settings.webhooks.api_key')}</label>
              <input
                type="text"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px] font-mono"
                placeholder={t('settings.webhooks.api_key_placeholder')}
                disabled={!enabled}
              />
              <p className="text-xs text-text-secondary mt-1">{t('settings.webhooks.api_key_hint')}</p>
            </div>
            <div className="flex justify-end">
              <button
                onClick={handleSave}
                disabled={saving}
                className="px-4 py-2 rounded-ios-sm bg-primary text-white font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
              >
                {saving ? '...' : t('app.save')}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}