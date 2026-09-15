import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { useI18nStore } from '../stores/i18nStore'

export default function SetupPage() {
  const navigate = useNavigate()
  const { setup } = useAuthStore()
  const { t } = useI18nStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (password !== passwordConfirm) {
      setError(t('setup.password_mismatch'))
      return
    }
    setLoading(true)
    try {
      await setup(email, password, passwordConfirm)
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
        <h1 className="text-xl sm:text-2xl font-bold mb-2">{t('setup.title')}</h1>
        <p className="text-text-secondary mb-6 text-sm">{t('setup.subtitle')}</p>
        {error && (
          <div className="bg-danger/10 text-danger text-sm p-3 rounded-ios-sm mb-4">
            {error}
          </div>
        )}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">{t('setup.email')}</label>
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
            <label className="block text-sm font-medium mb-1">{t('setup.password')}</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">{t('setup.password_confirm')}</label>
            <input
              type="password"
              value={passwordConfirm}
              onChange={(e) => setPasswordConfirm(e.target.value)}
              className="w-full px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px]"
              required
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-primary text-white py-2.5 rounded-ios-sm font-medium hover:bg-primary-hover disabled:opacity-50 transition-colors min-h-[44px]"
          >
            {loading ? '...' : t('setup.submit')}
          </button>
        </form>
      </div>
    </div>
  )
}