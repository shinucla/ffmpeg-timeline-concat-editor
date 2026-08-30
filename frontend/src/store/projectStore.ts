import { create } from 'zustand'
import type { TimelineSegment, VideoSummary } from '../types'
import { insertSegmentByLibraryOrder, sortSegmentsByLibraryOrder } from '../utils/segmentOrder'
import { findVideoByFilename, type TimelineEntry } from '../utils/timeline'
import { effectiveSegmentRepeat } from '../utils/time'
import { useLibraryStore } from './libraryStore'

interface ProjectState {
  segments: TimelineSegment[]
  selectedVideoId: string | null
  activeSegmentId: string | null
  outputName: string
  cacheFolder: string | null
  selectVideo: (videoId: string | null) => void
  selectSegment: (segmentId: string | null) => void
  addSegment: (video: VideoSummary, start: number, end: number) => string
  updateSegment: (segmentId: string, start: number, end: number) => void
  adjustSegmentRepeat: (segmentId: string, delta: number) => void
  setSegmentRepeat: (segmentId: string, repeat: number) => void
  setSegmentRotationSteps: (segmentId: string, rotationSteps: number) => void
  reorderSegments: (activeId: string, overId: string) => void
  removeSegment: (segmentId: string) => void
  setOutputName: (name: string) => void
  setCacheFolder: (cacheFolder: string | null) => void
  loadFromTimeline: (
    entries: TimelineEntry[],
    videos: VideoSummary[],
    outputName?: string,
    cacheFolder?: string | null,
  ) => { loaded: number; missing: string[] }
  clearProject: () => void
}

function reorder(segments: TimelineSegment[]): TimelineSegment[] {
  return segments.map((seg, order) => ({ ...seg, order }))
}

function moveSegmentInOrder(
  segments: TimelineSegment[],
  activeId: string,
  overId: string,
): TimelineSegment[] {
  const sorted = [...segments].sort((a, b) => a.order - b.order)
  const fromIndex = sorted.findIndex((seg) => seg.id === activeId)
  const toIndex = sorted.findIndex((seg) => seg.id === overId)
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return segments
  }
  const next = [...sorted]
  const [moved] = next.splice(fromIndex, 1)
  next.splice(toIndex, 0, moved)
  return reorder(next)
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  segments: [],
  selectedVideoId: null,
  activeSegmentId: null,
  outputName: 'final-output',
  cacheFolder: null,

  selectVideo: (videoId) =>
    set({
      selectedVideoId: videoId,
      activeSegmentId: null,
    }),

  selectSegment: (segmentId) => {
    if (!segmentId) {
      set({ activeSegmentId: null })
      return
    }
    const seg = get().segments.find((s) => s.id === segmentId)
    if (!seg) return
    set({
      selectedVideoId: seg.videoId,
      activeSegmentId: segmentId,
    })
  },

  addSegment: (video, start, end) => {
    const id = crypto.randomUUID()
    const newSegment: TimelineSegment = {
      id,
      videoId: video.id,
      video,
      start,
      end,
      order: 0,
      repeat: 1,
      rotationSteps: 0,
    }
    set((state) => ({
      segments: insertSegmentByLibraryOrder(
        state.segments,
        newSegment,
        useLibraryStore.getState().videos,
      ),
      selectedVideoId: video.id,
      activeSegmentId: id,
    }))
    return id
  },

  updateSegment: (segmentId, start, end) =>
    set((state) => {
      const current = state.segments.find((seg) => seg.id === segmentId)
      if (!current || (current.start === start && current.end === end)) {
        return state
      }
      return {
        segments: state.segments.map((seg) =>
          seg.id === segmentId ? { ...seg, start, end } : seg,
        ),
      }
    }),

  adjustSegmentRepeat: (segmentId, delta) =>
    set((state) => ({
      segments: state.segments.map((seg) => {
        if (seg.id !== segmentId) return seg
        return { ...seg, repeat: Math.max(1, effectiveSegmentRepeat(seg.repeat) + delta) }
      }),
    })),

  setSegmentRepeat: (segmentId, repeat) =>
    set((state) => ({
      segments: state.segments.map((seg) => {
        if (seg.id !== segmentId) return seg
        const next = Number.isFinite(repeat) ? Math.max(1, Math.floor(repeat)) : 1
        return { ...seg, repeat: next }
      }),
    })),

  setSegmentRotationSteps: (segmentId, rotationSteps) =>
    set((state) => ({
      segments: state.segments.map((seg) => {
        if (seg.id !== segmentId) return seg
        const parsed = Number.isFinite(rotationSteps) ? Math.floor(rotationSteps) : 0
        const next = ((parsed % 4) + 4) % 4
        return { ...seg, rotationSteps: next }
      }),
    })),

  reorderSegments: (activeId, overId) =>
    set((state) => {
      const active = state.segments.find((seg) => seg.id === activeId)
      const over = state.segments.find((seg) => seg.id === overId)
      if (!active || !over || active.videoId !== over.videoId) {
        return state
      }
      const moved = moveSegmentInOrder(state.segments, activeId, overId)
      return {
        segments: sortSegmentsByLibraryOrder(moved, useLibraryStore.getState().videos),
      }
    }),

  removeSegment: (segmentId) =>
    set((state) => {
      const filtered = state.segments.filter((seg) => seg.id !== segmentId)
      const next = sortSegmentsByLibraryOrder(
        filtered,
        useLibraryStore.getState().videos,
      )
      let activeSegmentId = state.activeSegmentId
      if (activeSegmentId === segmentId) {
        activeSegmentId = next.find((seg) => seg.videoId === state.selectedVideoId)?.id ?? null
      }
      return { segments: next, activeSegmentId }
    }),

  setOutputName: (name) => set({ outputName: name }),

  setCacheFolder: (cacheFolder) => set({ cacheFolder }),

  loadFromTimeline: (entries, videos, outputName, cacheFolder = null) => {
    const segments: TimelineSegment[] = []
    const missing: string[] = []

    for (const entry of entries) {
      const video = findVideoByFilename(videos, entry.filename)
      if (!video) {
        if (!missing.includes(entry.filename)) {
          missing.push(entry.filename)
        }
        continue
      }
      segments.push({
        id: crypto.randomUUID(),
        videoId: video.id,
        video,
        start: entry.start,
        end: entry.end,
        order: segments.length,
        repeat: entry.repeat,
        rotationSteps: entry.rotationSteps,
      })
    }

    const ordered = reorder(segments)

    set({
      segments: ordered,
      selectedVideoId: ordered[0]?.videoId ?? null,
      activeSegmentId: ordered[0]?.id ?? null,
      outputName: outputName ?? 'final-output',
      cacheFolder: cacheFolder ?? null,
    })

    return { loaded: ordered.length, missing }
  },

  clearProject: () =>
    set({
      segments: [],
      selectedVideoId: null,
      activeSegmentId: null,
      outputName: 'final-output',
      cacheFolder: null,
    }),
}))
