export interface VideoSummary {
  id: string
  name: string
  path: string
  size: number
  modifiedAt: number
  duration: number
  width: number
  height: number
  fps: number
}

export type VideoSort =
  | 'name-asc'
  | 'name-desc'
  | 'size-asc'
  | 'size-desc'
  | 'modified-asc'
  | 'modified-desc'

/**
 * Burned-in text watermark for a segment.
 * Position/size are fractions (0..1) of the output frame, with (x, y) at the
 * top-left of the watermark box. `start`/`duration` are seconds relative to the
 * start of the owning segment (and apply to every repeat of that segment).
 */
export interface Watermark {
  enabled: boolean
  text: string
  start: number
  duration: number
  x: number
  y: number
  width: number
  height: number
}

export interface Segment {
  start: number
  end: number
  repeat?: number
  rotationSteps?: number
  alternateRepeatReverse?: boolean
}

export interface TimelineSegment {
  id: string
  videoId: string
  video?: VideoSummary
  start: number
  end: number
  order: number
  repeat: number
  rotationSteps: number
  alternateRepeatReverse: boolean
  watermark: Watermark
}

export interface ProjectClip {
  id: string
  videoId: string
  video?: VideoSummary
  order: number
  segments: Segment[]
}

export interface Job {
  id: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  progress: number
  message: string
  output?: string
  manifest?: string
  cacheFolder?: string
  error?: string
}

export interface AppConfig {
  videoRoot: string
  outputDir: string
}
