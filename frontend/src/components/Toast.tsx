import { useState, useEffect, createContext, useContext, useCallback } from 'react'
import { Icon } from './Icons'

interface Toast {
  id: number
  message: string
  type: 'error' | 'success' | 'info'
}

interface ToastContextType {
  showToast: (message: string, type?: 'error' | 'success' | 'info') => void
}

const ToastContext = createContext<ToastContextType>({ showToast: () => {} })

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const showToast = useCallback((message: string, type: 'error' | 'success' | 'info' = 'info') => {
    const id = Date.now()
    setToasts(prev => [...prev, { id, message, type }])
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id))
    }, 4000)
  }, [])

  const removeToast = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
        {toasts.map(toast => (
          <div
            key={toast.id}
            className={`flex items-center gap-3 px-4 py-3 rounded-ios shadow-ios text-sm font-medium animate-slide-in ${
              toast.type === 'error' ? 'bg-red-50 text-red-800 border border-red-200 dark:bg-red-950/40 dark:text-red-300 dark:border-red-900' :
              toast.type === 'success' ? 'bg-green-50 text-green-800 border border-green-200 dark:bg-green-950/40 dark:text-green-300 dark:border-green-900' :
              'bg-blue-50 text-blue-800 border border-blue-200 dark:bg-blue-950/40 dark:text-blue-300 dark:border-blue-900'
            }`}
          >
            <Icon name={toast.type === 'error' ? 'warning' : toast.type === 'success' ? 'save' : 'other'} className="w-4 h-4 flex-shrink-0" />
            <span className="flex-1">{toast.message}</span>
            <button onClick={() => removeToast(toast.id)} className="text-current opacity-60 hover:opacity-100">
              <Icon name="cancel" className="w-3 h-3" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
