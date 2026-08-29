import { create } from 'zustand'

const STORAGE_KEY = 'video-cut-player-settings'

export interface PlayerSettings {
  volume: number
  playbackRate: number
  muted: boolean
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function loadSettings(): PlayerSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return { volume: 1, playbackRate: 1, muted: false }
    }
    const parsed = JSON.parse(raw) as Partial<PlayerSettings>
    return {
      volume: clamp(typeof parsed.volume === 'number' ? parsed.volume : 1, 0, 1),
      playbackRate: clamp(
        typeof parsed.playbackRate === 'number' ? parsed.playbackRate : 1,
        0.25,
        4,
      ),
      muted: Boolean(parsed.muted),
    }
  } catch {
    return { volume: 1, playbackRate: 1, muted: false }
  }
}

function saveSettings(settings: PlayerSettings) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
  } catch {
    // Ignore quota / private mode errors.
  }
}

interface PlayerState extends PlayerSettings {
  applyTo: (video: HTMLVideoElement) => void
  syncFrom: (video: HTMLVideoElement) => void
}

export const usePlayerStore = create<PlayerState>((set, get) => ({
  ...loadSettings(),

  applyTo: (video) => {
    const { volume, playbackRate, muted } = get()
    video.volume = volume
    video.playbackRate = playbackRate
    video.muted = muted
  },

  syncFrom: (video) => {
    const settings: PlayerSettings = {
      volume: video.volume,
      playbackRate: video.playbackRate,
      muted: video.muted,
    }
    set(settings)
    saveSettings(settings)
  },
}))
