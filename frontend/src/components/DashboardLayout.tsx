import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { useI18nStore } from '../stores/i18nStore'
import { useAuthStore } from '../stores/authStore'
import { usePageTitleStore } from '../stores/pageTitleStore'
import { Icon } from '../components/Icons'
import Sidebar from '../components/Sidebar'

// Rutas de detalle: cuando la ruta actual coincide, la hamburguesa se reemplaza
// por una flecha atrás que navega a la lista padre.
const BACK_ROUTES: Record<string, string> = {
  '/apps/:appId': '/apps',
  '/apps/:appId/accounts/:accountId/bills': '/apps/:appId',
}

function resolveBackRoute(pathname: string): string | null {
  for (const [pattern, to] of Object.entries(BACK_ROUTES)) {
    const patternParts = pattern.split('/').filter(Boolean)
    const pathParts = pathname.split('/').filter(Boolean)
    if (patternParts.length !== pathParts.length) continue
    let matches = true
    for (let i = 0; i < patternParts.length; i++) {
      if (patternParts[i].startsWith(':') || patternParts[i] === pathParts[i]) continue
      matches = false
      break
    }
    if (matches) return to
  }
  return null
}

export default function DashboardLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useI18nStore()
  const { logout } = useAuthStore()
  const pageTitle = usePageTitleStore((s) => s.title)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const activeBase = location.pathname.split('/')[1] || 'dashboard'
  const backRoute = resolveBackRoute(location.pathname)

  const handleLogout = async () => {
    await logout()
  }

  return (
    <div className="flex min-h-screen">
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/30 z-40 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
      <Sidebar
        activeBase={activeBase}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
      />
      <div className="flex-1 ml-0 lg:ml-60 flex flex-col min-h-screen">
        <header className="h-14 bg-card border-b border-border flex items-center justify-between px-3 sm:px-5 sticky top-0 z-50">
          <div className="flex items-center gap-2">
            {backRoute ? (
              <button
                onClick={() => navigate(backRoute)}
                className="lg:hidden w-10 h-10 rounded-full flex items-center justify-center hover:bg-bg transition-colors min-h-[44px]"
                title={t('app.back')}
              >
                <Icon name="back" className="w-5 h-5" />
              </button>
            ) : (
              <button
                onClick={() => setSidebarOpen(true)}
                className="lg:hidden w-10 h-10 rounded-full flex items-center justify-center hover:bg-bg transition-colors min-h-[44px]"
                title={t('menu.dashboard')}
              >
                <Icon name="menu" className="w-5 h-5" />
              </button>
            )}
            <h1 className="text-base sm:text-lg font-semibold min-w-0 truncate">{pageTitle ?? t(`${activeBase}.title`, t('app.title'))}</h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/settings')}
              className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
              title={t('menu.settings')}
            >
              <Icon name="settings" className="w-5 h-5" />
            </button>
            <button
              onClick={handleLogout}
              className="w-9 h-9 rounded-full flex items-center justify-center hover:bg-bg transition-colors"
              title={t('app.close')}
            >
              <Icon name="logout" className="w-5 h-5" />
            </button>
          </div>
        </header>
        <main className="flex-1 p-3 sm:p-5 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}