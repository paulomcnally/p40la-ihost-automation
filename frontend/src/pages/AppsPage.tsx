import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { useToast } from '../components/Toast'
import { Icon } from '../components/Icons'
import { api, type App } from '../api'

interface AppModalProps {
  initial?: App | null
  onClose: () => void
  onSaved: (app: App) => void
}

function AppModal({ initial, onClose, onSaved }: AppModalProps) {
  const { t } = useI18nStore()
  const { showToast } = useToast()
  const [name, setName] = useState(initial?.name ?? '')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const app = initial
        ? await api.apps.update(initial.id, name)
        : await api.apps.create(name)
      if (!app) throw new Error(t('errors.generic'))
      showToast(t('apps.saved'), 'success')
      onSaved(app)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('errors.generic'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/30 z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div className="bg-card rounded-ios shadow-ios p-5 w-full max-w-md" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-semibold mb-4">{initial ? t('apps.edit') : t('apps.create')}</h3>
        {error && (
          <div className="bg-danger/10 text-danger text-sm p-3 rounded-ios-sm mb-4">{error}</div>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">{t('apps.name')}</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              placeholder={t('apps.name_placeholder')}
              required
              autoFocus
            />
          </div>
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
              disabled={loading}
              className="px-4 py-2 rounded-ios-sm bg-primary text-white font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
            >
              {loading ? '...' : t('app.save')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default function AppsPage() {
  const navigate = useNavigate()
  const { t } = useI18nStore()
  const { showToast } = useToast()
  usePageTitle(t('apps.title'))

  const [apps, setApps] = useState<App[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<App | null>(null)

  const loadApps = async () => {
    setLoading(true)
    try {
      const data = await api.apps.list()
      setApps(data ?? [])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadApps() }, [])

  const handleDelete = async (app: App) => {
    if (!window.confirm(t('apps.delete_confirm'))) return
    try {
      await api.apps.remove(app.id)
      showToast(t('apps.deleted'), 'success')
      loadApps()
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    }
  }

  const handleSaved = (saved: App) => {
    setModalOpen(false)
    setEditing(null)
    loadApps()
  }

  return (
    <div className="max-w-2xl mx-auto">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-xl sm:text-2xl font-bold">{t('apps.title')}</h2>
        <button
          onClick={() => { setEditing(null); setModalOpen(true) }}
          className="flex items-center gap-2 bg-primary text-white px-4 py-2 rounded-ios-sm font-medium hover:bg-primary-hover transition-colors min-h-[44px]"
        >
          <Icon name="plus" className="w-4 h-4" />
          {t('apps.create')}
        </button>
      </div>
      <p className="text-text-secondary mb-6">{t('apps.subtitle')}</p>

      {loading ? (
        <div className="text-text-secondary py-10 text-center">{t('app.loading')}</div>
      ) : apps.length === 0 ? (
        <div className="bg-card rounded-ios shadow-ios p-10 flex flex-col items-center justify-center text-center">
          <Icon name="services" className="w-12 h-12 text-text-secondary/40 mb-3" />
          <p className="text-text-secondary">{t('apps.empty')}</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {apps.map((app) => (
            <li key={app.id}>
              <div className="bg-card rounded-ios shadow-ios p-4 flex items-center justify-between gap-3">
                <button
                  onClick={() => navigate(`/apps/${app.id}`)}
                  className="flex-1 min-w-0 text-left min-h-[44px] flex items-center gap-3"
                >
                  <Icon name="services" className="w-5 h-5 text-primary flex-shrink-0" />
                  <span className="min-w-0">
                    <span className="block truncate font-medium">{app.name}</span>
                    <span className="block text-sm text-text-secondary">
                      {app.account_count ?? 0} {t('apps.account_count')}
                    </span>
                  </span>
                  <Icon name="chevron" className="w-4 h-4 text-text-secondary flex-shrink-0" />
                </button>
                <div className="flex items-center gap-1 flex-shrink-0">
                  <button
                    onClick={() => { setEditing(app); setModalOpen(true) }}
                    className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
                    title={t('apps.edit')}
                  >
                    <Icon name="edit" className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(app)}
                    className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
                    title={t('apps.delete')}
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
        <AppModal
          initial={editing}
          onClose={() => { setModalOpen(false); setEditing(null) }}
          onSaved={handleSaved}
        />
      )}
    </div>
  )
}