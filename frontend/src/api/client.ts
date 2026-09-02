import type { AppConfig, Job, TimelineSegment, VideoSummary } from '../types'
import type { Segment } from '../types'

const API = '/api'

function segmentsToExportClips(segments: TimelineSegment[]) {
  return [...segments]
    .sort((a, b) => a.order - b.order)
    .map((seg, i) => ({
      id: seg.id,
      videoId: seg.videoId,
      order: i,
      segments: [{
        start: seg.start,
        end: seg.end,
        repeat: seg.repeat,
        rotationSteps: seg.rotationSteps,
        alternateRepeatReverse: seg.alternateRepeatReverse,
      }],
    }))
}

export async function fetchConfig(): Promise<AppConfig> {
  const res = await fetch(`${API}/config`)
  if (!res.ok) throw new Error('Failed to load config')
  return res.json()
}

export async function fetchVideos(): Promise<VideoSummary[]> {
  const res = await fetch(`${API}/videos`)
  if (!res.ok) throw new Error('Failed to load videos')
  return res.json()
}

export async function setVideoRoot(videoRoot: string): Promise<AppConfig> {
  const res = await fetch(`${API}/config/video-root`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ videoRoot }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to set video folder')
  }
  return res.json()
}

export async function browseVideoFolder(): Promise<AppConfig> {
  const res = await fetch(`${API}/config/browse-folder`, { method: 'POST' })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to open folder picker')
  }
  return res.json()
}

export function streamUrl(videoId: string): string {
  return `${API}/videos/${encodeURIComponent(videoId)}/stream`
}

export function frameUrl(videoId: string, t: number): string {
  return `${API}/videos/${encodeURIComponent(videoId)}/frame?t=${t}`
}

export async function startProcess(
  name: string,
  segments: TimelineSegment[],
  exportCutParts = false,
): Promise<{ jobId: string }> {
  const res = await fetch(`${API}/process`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name,
      exportCutParts,
      clips: segmentsToExportClips(segments),
    }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Process failed')
  }
  return res.json()
}

export async function fetchJob(jobId: string): Promise<Job> {
  const res = await fetch(`${API}/jobs/${jobId}`)
  if (!res.ok) throw new Error('Job not found')
  return res.json()
}

export async function saveTimeline(
  name: string,
  segments: TimelineSegment[],
  cacheFolder: string | null,
): Promise<{ path: string }> {
  const res = await fetch(`${API}/timeline/save`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name,
      cacheFolder: cacheFolder ?? '',
      segments: [...segments]
        .sort((a, b) => a.order - b.order)
        .map((seg) => ({
          start: seg.start,
          end: seg.end,
          filename: seg.video?.name ?? '',
          repeat: seg.repeat,
          rotationSteps: seg.rotationSteps,
          alternateRepeatReverse: seg.alternateRepeatReverse,
        })),
    }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to save timeline')
  }
  return res.json()
}

export function formatDuration(seconds: number): string {
  if (!seconds || seconds < 0) return '0:00'
  const total = Math.max(0, Math.round(seconds))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
  }
  return `${m}:${s.toString().padStart(2, '0')}`
}

export function formatDurationClock(seconds: number): string {
  if (!seconds || seconds < 0) return '00:00:00'
  const total = Math.max(0, Math.round(seconds))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

export function formatSegmentBarDuration(seconds: number): string {
  if (!seconds || seconds < 0) return '0sec'
  const total = Math.max(0, Math.round(seconds))
  if (total < 60) {
    return `${total}sec`
  }
  return formatDurationClock(seconds)
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

export type { Segment }
