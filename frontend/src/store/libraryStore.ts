import { create } from 'zustand'
import {
  browseVideoFolder,
  fetchConfig,
  fetchVideos,
  setVideoRoot,
} from '../api/client'
import type { VideoSummary } from '../types'
import { useProjectStore } from './projectStore'

interface LibraryState {
  videos: VideoSummary[]
  folderInput: string
  loading: boolean
  switching: boolean
  folderError: string | null
  setFolderInput: (path: string) => void
  loadLibrary: () => Promise<void>
  applyFolder: (path: string) => Promise<void>
  browseFolder: () => Promise<void>
}

export const useLibraryStore = create<LibraryState>((set) => ({
  videos: [],
  folderInput: '',
  loading: true,
  switching: false,
  folderError: null,

  setFolderInput: (path) => set({ folderInput: path }),

  loadLibrary: async () => {
    set({ loading: true, folderError: null })
    try {
      const [videos, cfg] = await Promise.all([fetchVideos(), fetchConfig()])
      set({ videos, folderInput: cfg.videoRoot, loading: false })
    } catch (e) {
      set({
        loading: false,
        folderError: e instanceof Error ? e.message : 'Failed to load library',
      })
    }
  },

  applyFolder: async (path) => {
    set({ switching: true, folderError: null })
    try {
      const cfg = await setVideoRoot(path)
      useProjectStore.getState().clearProject()
      const videos = await fetchVideos()
      set({ folderInput: cfg.videoRoot, videos, switching: false })
    } catch (e) {
      set({
        switching: false,
        folderError: e instanceof Error ? e.message : 'Failed to set folder',
      })
    }
  },

  browseFolder: async () => {
    set({ switching: true, folderError: null })
    try {
      const cfg = await browseVideoFolder()
      useProjectStore.getState().clearProject()
      const videos = await fetchVideos()
      set({ folderInput: cfg.videoRoot, videos, switching: false })
    } catch (e) {
      set({
        switching: false,
        folderError: e instanceof Error ? e.message : 'Failed to browse folder',
      })
    }
  },
}))
