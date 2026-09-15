import { create } from 'zustand'
import { api } from '../api'

interface AuthState {
  isAuthenticated: boolean
  isSetupComplete: boolean | null
  checkSetup: () => Promise<void>
  checkSession: () => Promise<void>
  login: (email: string, password: string, remember: boolean) => Promise<void>
  logout: () => Promise<void>
  setup: (email: string, password: string, password_confirm: string) => Promise<void>
}

export const useAuthStore = create<AuthState>((set) => ({
  isAuthenticated: false,
  isSetupComplete: null,

  checkSetup: async () => {
    const status = await api.auth.setupStatus()
    set({ isSetupComplete: (status as any)?.setup_completed ?? false })
  },

  checkSession: async () => {
    try {
      const res = await fetch('/api/me', { credentials: 'same-origin' })
      if (res.ok) set({ isAuthenticated: true })
      else set({ isAuthenticated: false })
    } catch {
      set({ isAuthenticated: false })
    }
  },

  login: async (email: string, password: string, remember: boolean) => {
    await api.auth.login(email, password, remember)
    set({ isAuthenticated: true })
  },

  logout: async () => {
    await api.auth.logout()
    set({ isAuthenticated: false })
    window.location.href = '/login'
  },

  setup: async (email: string, password: string, password_confirm: string) => {
    await api.auth.setup(email, password, password_confirm)
    set({ isAuthenticated: true, isSetupComplete: true })
  },
}))