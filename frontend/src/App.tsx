import { useEffect, useState } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './stores/authStore'
import { useI18nStore } from './stores/i18nStore'
import { ToastProvider } from './components/Toast'
import DashboardLayout from './components/DashboardLayout'
import LoginPage from './pages/LoginPage'
import SetupPage from './pages/SetupPage'
import DashboardPage from './pages/DashboardPage'
import SettingsPage from './pages/SettingsPage'
import AppsPage from './pages/AppsPage'
import AppDetailPage from './pages/AppDetailPage'
import PluginsPage from './pages/PluginsPage'
import AccountBillsPage from './pages/AccountBillsPage'

function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

function App() {
  const { checkSetup, checkSession, isSetupComplete } = useAuthStore()
  const { lang, load } = useI18nStore()
  const [initialized, setInitialized] = useState(false)

  useEffect(() => {
    const init = async () => {
      await checkSetup()
      await checkSession()
      await load(lang)
      setInitialized(true)
    }
    init()
  }, [])

  if (!initialized) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-bg">
        <div className="text-text-secondary text-lg">Loading...</div>
      </div>
    )
  }

  if (isSetupComplete === false) {
    return <SetupPage />
  }

  return (
    <ToastProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/*"
          element={
            <AuthGuard>
              <Routes>
                <Route path="/" element={<DashboardLayout />}>
                  <Route index element={<Navigate to="/dashboard" replace />} />
                  <Route path="dashboard" element={<DashboardPage />} />
                  <Route path="apps" element={<AppsPage />} />
                  <Route path="apps/:appId" element={<AppDetailPage />} />
                  <Route path="apps/:appId/accounts/:accountId/bills" element={<AccountBillsPage />} />
                  <Route path="plugins" element={<PluginsPage />} />
                  <Route path="settings" element={<SettingsPage />} />
                </Route>
              </Routes>
            </AuthGuard>
          }
        />
      </Routes>
    </ToastProvider>
  )
}

export default App