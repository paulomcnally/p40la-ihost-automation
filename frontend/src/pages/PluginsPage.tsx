import { useEffect, useState } from 'react'
import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { useToast } from '../components/Toast'
import { Icon } from '../components/Icons'
import { api, type Plugin } from '../api'

export default function PluginsPage() {
  const { t } = useI18nStore()
  const { showToast } = useToast()
  usePageTitle(t('plugins.title'))

  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [loading, setLoading] = useState(true)

  const loadPlugins = async () => {
    setLoading(true)
    try {
      const data = await api.plugins.list()
      setPlugins(data ?? [])
    } catch (err: unknown) {
      showToast(err instanceof Error ? err.message : t('errors.generic'), 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadPlugins() }, [])

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-xl sm:text-2xl font-bold mb-2">{t('plugins.title')}</h2>
      <p className="text-text-secondary mb-6">{t('plugins.subtitle')}</p>

      {loading ? (
        <div className="text-text-secondary py-10 text-center">{t('app.loading')}</div>
      ) : plugins.length === 0 ? (
        <div className="bg-card rounded-ios shadow-ios p-10 flex flex-col items-center justify-center text-center">
          <Icon name="globe" className="w-12 h-12 text-text-secondary/40 mb-3" />
          <p className="text-text-secondary">{t('plugins.empty')}</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {plugins.map((plugin) => (
            <li key={plugin.id}>
              <div className="bg-card rounded-ios shadow-ios p-4">
                <div className="flex items-center gap-3 flex-wrap">
                  <Icon name="globe" className="w-5 h-5 text-primary flex-shrink-0" />
                  <span className="font-medium truncate">{plugin.name}</span>
                  <span className="bg-bg text-text-secondary text-sm px-2 py-0.5 rounded-ios-sm font-mono">
                    v{plugin.version}
                  </span>
                  <span className={`text-xs px-2 py-0.5 rounded-ios-sm ${
                    plugin.status === 'active'
                      ? 'bg-primary/10 text-primary'
                      : 'bg-danger/10 text-danger'
                  }`}>
                    {t(`plugins.status.${plugin.status}`)}
                  </span>
                </div>
                {plugin.description && (
                  <p className="mt-2 text-sm text-text-secondary">{plugin.description}</p>
                )}
                <p className="mt-1 text-xs text-text-secondary font-mono">{plugin.source}</p>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}