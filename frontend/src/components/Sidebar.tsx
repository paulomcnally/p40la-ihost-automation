import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useI18nStore } from '../stores/i18nStore'
import { Icon } from './Icons'

interface SidebarProps {
  activeBase: string
  isOpen?: boolean
  onClose?: () => void
}

export default function Sidebar({ activeBase, isOpen, onClose }: SidebarProps) {
  const navigate = useNavigate()
  const { t } = useI18nStore()

  const items = [
    { key: 'dashboard', icon: 'home', label: t('menu.dashboard') },
    { key: 'apps', icon: 'services', label: t('menu.apps') },
    { key: 'plugins', icon: 'globe', label: t('menu.plugins') },
    { key: 'settings', icon: 'settings', label: t('menu.settings') },
  ]

  const handleNavigate = (key: string) => {
    navigate(`/${key}`)
    onClose?.()
  }

  return (
    <aside className={`
      bg-card border-border flex flex-col
      fixed top-0 bottom-0 z-50
      w-60 left-0
      lg:fixed lg:left-0 lg:top-0 lg:bottom-0
      border-r
      transition-transform duration-200 ease-in-out
      ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
    `}>
      <div className="px-5 py-4 text-xl font-bold text-primary border-b border-border">
        {t('app.title')}
      </div>
      <nav className="flex-1 p-3 overflow-y-auto">
        <div className="mb-4">
          <div className="text-xs uppercase tracking-wide text-text-secondary font-semibold px-3 py-2">
            {t('app.title')}
          </div>
          {items.map((item) => (
            <button
              key={item.key}
              onClick={() => handleNavigate(item.key)}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-ios-sm text-sm transition-colors min-h-[44px] ${
                activeBase === item.key
                  ? 'bg-primary/10 text-primary font-medium'
                  : 'hover:bg-bg'
              }`}
            >
              <Icon name={item.icon} className="w-5 h-5" />
              <span>{item.label}</span>
            </button>
          ))}
        </div>
      </nav>
    </aside>
  )
}