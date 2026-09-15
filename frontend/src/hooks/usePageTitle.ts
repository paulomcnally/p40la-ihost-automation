import { useEffect } from 'react'
import { usePageTitleStore } from '../stores/pageTitleStore'

export function usePageTitle(title: string | null) {
  const setTitle = usePageTitleStore((s) => s.setTitle)
  useEffect(() => {
    setTitle(title)
    return () => setTitle(null)
  }, [title, setTitle])
}