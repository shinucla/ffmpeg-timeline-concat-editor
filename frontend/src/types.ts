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

export interface Segment {
  start: number
  end: number
  repeat?: number
  rotationSteps?: number
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
