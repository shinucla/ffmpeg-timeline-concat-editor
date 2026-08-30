import type { TimelineSegment } from '../types'
import { quantizeToFrame } from './time'

export const DEFAULT_NEW_SEGMENT_LEN = 30
export const SNAP_PX = 10

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

export function segmentsSortedByTime(segs: TimelineSegment[]) {
  return [...segs].sort((a, b) => a.start - b.start || a.order - b.order)
}

export function getOtherSegments(segmentId: string, segs: TimelineSegment[]) {
  return segs.filter((seg) => seg.id !== segmentId)
}

export function collectSegmentEdgeSnapTargets(others: TimelineSegment[]) {
  const targets: number[] = []
  for (const seg of others) {
    targets.push(seg.start, seg.end)
  }
  return targets
}

export function snapThresholdSec(trackWidth: number, duration: number) {
  if (trackWidth <= 0 || duration <= 0) return 0
  return (SNAP_PX / trackWidth) * duration
}

export function snapTimeToNearest(
  time: number,
  targets: number[],
  trackWidth: number,
  duration: number,
): number {
  const threshold = snapThresholdSec(trackWidth, duration)
  if (threshold <= 0 || targets.length === 0) return time

  let best = time
  let bestDist = threshold + 1
  for (const target of targets) {
    const dist = Math.abs(time - target)
    if (dist <= threshold && dist < bestDist) {
      bestDist = dist
      best = target
    }
  }
  return best
}

export type SegmentDragMode = 'resize-start' | 'resize-end' | 'move'

export function clampSegmentToBounds(
  start: number,
  end: number,
  duration: number,
  minDuration: number,
  mode: SegmentDragMode,
): { start: number; end: number } {
  let s = start
  let e = end
  const len = Math.max(minDuration, end - start)

  if (mode === 'move') {
    s = clamp(s, 0, Math.max(0, duration - len))
    e = s + len
    return { start: s, end: e }
  }

  if (mode === 'resize-start') {
    s = clamp(s, 0, end - minDuration)
    return { start: s, end: e }
  }

  e = clamp(e, start + minDuration, duration)
  return { start: s, end: e }
}

export function applySegmentDragUpdate(
  mode: SegmentDragMode,
  origin: { start: number; end: number },
  delta: number,
  playhead: number,
  trackWidth: number,
  duration: number,
  minDuration: number,
  fps: number,
  others: TimelineSegment[],
) {
  let start = origin.start
  let end = origin.end
  const edgeTargets = collectSegmentEdgeSnapTargets(others)

  if (mode === 'resize-start') {
    start = clamp(origin.start + delta, 0, origin.end - minDuration)
    const allowPlayheadSnap = !(delta > 0 && playhead <= origin.start)
    const targets: number[] = [...edgeTargets]
    if (allowPlayheadSnap) targets.push(playhead)
    start = snapTimeToNearest(start, targets, trackWidth, duration)
    start = clamp(start, 0, origin.end - minDuration)
  } else if (mode === 'resize-end') {
    end = clamp(origin.end + delta, origin.start + minDuration, duration)
    const allowPlayheadSnap = !(delta < 0 && playhead >= origin.end)
    const targets: number[] = [...edgeTargets]
    if (allowPlayheadSnap) targets.push(playhead)
    end = snapTimeToNearest(end, targets, trackWidth, duration)
    end = clamp(end, origin.start + minDuration, duration)
  } else if (mode === 'move') {
    const len = origin.end - origin.start
    start = clamp(origin.start + delta, 0, duration - len)
    end = start + len

    const allowStartPlayhead = !(delta > 0 && playhead <= origin.start)
    const allowEndPlayhead = !(delta < 0 && playhead >= origin.end)

    const startTargets: number[] = [...edgeTargets]
    if (allowStartPlayhead) startTargets.push(playhead)
    for (const seg of others) {
      startTargets.push(seg.start - len)
    }

    const endTargets: number[] = [...edgeTargets]
    if (allowEndPlayhead) endTargets.push(playhead)
    for (const seg of others) {
      endTargets.push(seg.end + len)
    }

    const snappedStart = snapTimeToNearest(start, startTargets, trackWidth, duration)
    const snappedEnd = snapTimeToNearest(end, endTargets, trackWidth, duration)

    const startMoved = Math.abs(snappedStart - start) <= snapThresholdSec(trackWidth, duration) + 1e-9
    const endMoved = Math.abs(snappedEnd - end) <= snapThresholdSec(trackWidth, duration) + 1e-9

    if (startMoved && (!endMoved || Math.abs(snappedStart - start) <= Math.abs(snappedEnd - end))) {
      start = snappedStart
      end = start + len
    } else if (endMoved) {
      end = snappedEnd
      start = end - len
    }
  } else {
    return null
  }

  const clamped = clampSegmentToBounds(start, end, duration, minDuration, mode)

  return {
    start: quantizeToFrame(clamped.start, fps),
    end: quantizeToFrame(clamped.end, fps),
  }
}

export function computeNewSegmentRange(
  sliderTime: number,
  videoSegs: TimelineSegment[],
  duration: number,
  minDuration: number,
  fps: number,
): { start: number; end: number } | null {
  if (duration <= 0) return null

  const sorted = segmentsSortedByTime(videoSegs)
  let start = sliderTime
  for (const seg of sorted) {
    if (sliderTime >= seg.start && sliderTime < seg.end) {
      start = Math.max(start, seg.end)
    }
  }
  start = quantizeToFrame(clamp(start, 0, duration), fps)
  if (start >= duration) return null

  const next = sorted.find((seg) => seg.start > start + 1e-9) ?? null
  let end = start + Math.min(DEFAULT_NEW_SEGMENT_LEN, duration - start)
  if (next && end > next.start && next.start - start >= minDuration - 1e-9) {
    end = next.start
  }
  end = quantizeToFrame(clamp(end, start + minDuration, duration), fps)

  if (end - start < minDuration - 1e-9) {
    return null
  }

  return { start, end }
}
