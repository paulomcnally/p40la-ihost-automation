import { create } from 'zustand'

interface PageTitleState {
  title: string | null
  setTitle: (title: string | null) => void
}

export const usePageTitleStore = create<PageTitleState>((set) => ({
  title: null,
  setTitle: (title) => set({ title }),
}))