import { useI18nStore } from '../stores/i18nStore'
import { usePageTitle } from '../hooks/usePageTitle'
import { Icon } from '../components/Icons'

export default function DashboardPage() {
  const { t } = useI18nStore()
  usePageTitle(t('dashboard.title'))

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-xl sm:text-2xl font-bold mb-2">{t('dashboard.title')}</h2>
      <p className="text-text-secondary mb-6">{t('dashboard.subtitle')}</p>

      <div className="bg-card rounded-ios shadow-ios p-10 flex flex-col items-center justify-center text-center">
        <Icon name="home" className="w-12 h-12 text-text-secondary/40 mb-3" />
        <p className="text-text-secondary">{t('dashboard.empty')}</p>
      </div>
    </div>
  )
}