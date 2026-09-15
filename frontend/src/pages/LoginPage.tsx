import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useI18nStore } from '../stores/i18nStore'

export default function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuthStore()
  const { t } = useI18nStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(email, password, remember)
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('errors.generic'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg flex items-center justify-center p-4">
      <div className="bg-card rounded-ios shadow-ios p-6 sm:p-8 w-full max-w-md mx-4">
        <h1 className="text-xl sm:text-2xl font-bold mb-6 text-center">{t('app.title')}</h1>
        <h2 className="text-lg font-semibold mb-4">{t('login.title')}</h2>
        {error && (
          <div className="bg-danger/10 text-danger text-sm p-3 rounded-ios-sm mb-4">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">{t('login.email')}</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              required
              autoFocus
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">{t('login.password')}</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              required
            />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember(e.target.checked)}
              className="rounded"
            />
            {t('login.remember')}
          </label>
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-primary text-white py-2.5 rounded-ios-sm font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
          >
            {loading ? '...' : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  )
}