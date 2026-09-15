import { useEffect, useRef, useState } from 'react'
import { Icon } from './Icons'

export interface DropdownOption {
  value: string
  label: string
}

interface DropdownProps {
  value: string
  onChange: (value: string) => void
  options: DropdownOption[]
  placeholder?: string
  disabled?: boolean
  className?: string
}

export default function Dropdown({ value, onChange, options, placeholder, disabled, className = '' }: DropdownProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const selected = options.find((o) => o.value === value)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const handleSelect = (option: DropdownOption) => {
    setOpen(false)
    if (option.value !== value) onChange(option.value)
  }

  return (
    <div ref={ref} className={`relative ${className}`}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        className={`w-full flex items-center justify-between gap-2 bg-card text-text px-3 py-2 border border-border rounded-ios-sm focus:outline-none focus:border-primary min-h-[44px] disabled:opacity-50 transition-colors ${
          selected ? '' : 'text-text-secondary'
        }`}
      >
        <span className="truncate">{selected ? selected.label : (placeholder ?? '')}</span>
        <Icon name="chevron" className={`w-4 h-4 text-text-secondary flex-shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} />
      </button>

      {open && (
        <ul
          role="listbox"
          className="absolute z-20 mt-1 w-full bg-card border border-border rounded-ios-sm shadow-ios overflow-hidden"
        >
          {options.map((option) => (
            <li key={option.value} role="option" aria-selected={option.value === value}>
              <button
                type="button"
                onClick={() => handleSelect(option)}
                className={`w-full text-left px-3 py-2.5 text-sm min-h-[44px] transition-colors hover:bg-bg ${
                  option.value === value ? 'text-primary font-medium bg-primary/5' : 'text-text'
                }`}
              >
                {option.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}