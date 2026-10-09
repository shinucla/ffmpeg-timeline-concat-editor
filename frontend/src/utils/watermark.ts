import type { Watermark } from '../types'

export const DEFAULT_WATERMARK_TEXT = 'Watermark'
export const MIN_WATERMARK_SIZE = 0.04
export const MIN_WATERMARK_DURATION = 0.1

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

/** Default watermark covering the whole segment, placed in the top-left corner. */
export function createDefaultWatermark(duration: number): Watermark {
  const segDuration = Math.max(0, duration)
  return {
    enabled: false,
    text: DEFAULT_WATERMARK_TEXT,
    start: 0,
    duration: segDuration,
    x: 0.08,
    y: 0.08,
    width: 0.32,
    height: 0.14,
  }
}

/** Coerce a watermark (e.g. parsed from a timeline file) into valid ranges. */
export function normalizeWatermark(
  watermark: Watermark,
  segmentDuration: number,
): Watermark {
  const segDuration = Math.max(0, segmentDuration)
  const width = clamp(watermark.width, MIN_WATERMARK_SIZE, 1)
  const height = clamp(watermark.height, MIN_WATERMARK_SIZE, 1)
  const x = clamp(watermark.x, 0, 1 - width)
  const y = clamp(watermark.y, 0, 1 - height)

  const start = clamp(watermark.start, 0, segDuration)
  let duration = watermark.duration
  if (!Number.isFinite(duration) || duration <= 0) {
    duration = Math.max(0, segDuration - start)
  }
  duration = clamp(duration, 0, segDuration - start)

  return {
    enabled: Boolean(watermark.enabled),
    text: watermark.text ?? '',
    start,
    duration,
    x,
    y,
    width,
    height,
  }
}
