import type { Segment, VideoSummary } from '../types'

export interface TimelineEntry {
  start: number
  end: number
  filename: string
  repeat: number
}

export interface ParsedTimeline {
  cacheFolder: string | null
  entries: TimelineEntry[]
}

const OLD_LINE_RE = /^(\d{2}:\d{2}:\d{2})\s*-\s*(\d{2}:\d{2}:\d{2})\s+(.+)$/
const CACHE_HEADER_RE = /^#\s*cache:\s*(\S+)\s*$/i

export function parseTimelineTime(value: string): number {
  const parts = value.trim().split(':').map(Number)
  if (parts.length !== 3 || parts.some((n) => Number.isNaN(n))) {
    throw new Error(`Invalid time: ${value}`)
  }
  const [h, m, s] = parts
  return h * 3600 + m * 60 + s
}

export function formatTimelineTime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

function parseTimelineLine(line: string, lineNumber: number): TimelineEntry {
  const oldMatch = line.match(OLD_LINE_RE)
  if (oldMatch) {
    const start = parseTimelineTime(oldMatch[1])
    const end = parseTimelineTime(oldMatch[2])
    if (end <= start) {
      throw new Error(`Invalid segment on line ${lineNumber}: end must be after start`)
    }
    return {
      start,
      end,
      filename: oldMatch[3].trim(),
      repeat: 1,
    }
  }

  const parts = line.trim().split(/\s+/)
  if (parts.length < 4) {
    throw new Error(`Invalid timeline line ${lineNumber}: ${line}`)
  }

  const start = parseTimelineTime(parts[0])
  const end = parseTimelineTime(parts[1])
  if (end <= start) {
    throw new Error(`Invalid segment on line ${lineNumber}: end must be after start`)
  }

  const repeatToken = parts[parts.length - 1]
  if (!/^\d+$/.test(repeatToken)) {
    throw new Error(`Invalid timeline line ${lineNumber}: repeat must be a number`)
  }

  const repeat = Number.parseInt(repeatToken, 10)
  if (repeat < 1) {
    throw new Error(`Invalid segment on line ${lineNumber}: repeat must be at least 1`)
  }

  const filename = parts.slice(2, -1).join(' ').trim()
  if (!filename) {
    throw new Error(`Invalid timeline line ${lineNumber}: missing filename`)
  }

  return { start, end, filename, repeat }
}

function parseCacheHeader(line: string): string | null {
  const match = line.match(CACHE_HEADER_RE)
  if (!match) return null
  const name = match[1].trim()
  return /^cache-[a-zA-Z0-9_-]+$/.test(name) ? name : null
}

export function parseTimelineText(text: string): ParsedTimeline {
  const entries: TimelineEntry[] = []
  let cacheFolder: string | null = null
  const lines = text.split(/\r?\n/)

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim()
    if (!line) continue

    if (line.startsWith('#')) {
      const parsedCache = parseCacheHeader(line)
      if (parsedCache) {
        cacheFolder = parsedCache
      }
      continue
    }

    entries.push(parseTimelineLine(line, i + 1))
  }

  if (entries.length === 0) {
    throw new Error('Timeline file has no segments')
  }

  return { cacheFolder, entries }
}

export interface TimelineClipGroup {
  filename: string
  segments: Segment[]
}

export function groupTimelineEntries(entries: TimelineEntry[]): TimelineClipGroup[] {
  const groups: TimelineClipGroup[] = []

  for (const entry of entries) {
    const last = groups[groups.length - 1]
    const segment: Segment = {
      start: entry.start,
      end: entry.end,
      repeat: entry.repeat,
    }
    if (last && last.filename === entry.filename) {
      last.segments.push(segment)
    } else {
      groups.push({
        filename: entry.filename,
        segments: [segment],
      })
    }
  }

  return groups
}

export function findVideoByFilename(
  videos: VideoSummary[],
  filename: string,
): VideoSummary | undefined {
  const exact = videos.filter((v) => v.name === filename)
  if (exact.length === 1) return exact[0]
  if (exact.length > 1) {
    return exact.find((v) => v.path === filename || v.path.endsWith('/' + filename))
  }
  return videos.find((v) => v.path === filename || v.path.endsWith('/' + filename))
}

export function outputNameFromTimelineFile(filename: string): string {
  return filename.replace(/\.txt$/i, '') || 'final-output'
}

export function serializeTimelineText(
  segments: Array<{
    start: number
    end: number
    filename: string
    repeat: number
    order: number
  }>,
  cacheFolder?: string | null,
): string {
  const lines: string[] = []
  if (cacheFolder) {
    lines.push(`# cache: ${cacheFolder}`)
  }
  const sorted = [...segments].sort((a, b) => a.order - b.order)
  for (const seg of sorted) {
    lines.push(
      `${formatTimelineTime(seg.start)} ${formatTimelineTime(seg.end)} ${seg.filename} ${seg.repeat}`,
    )
  }
  return `${lines.join('\n')}\n`
}
