export function frameStep(fps: number) {
  return fps > 0 ? 1 / fps : 1 / 30
}

export function quantizeToFrame(seconds: number, fps: number) {
  const step = frameStep(fps)
  return Math.round(seconds / step) * step
}

export function segmentDuration(start: number, end: number) {
  return Math.max(0, end - start)
}

export function effectiveSegmentRepeat(repeat: number | undefined) {
  if (!Number.isFinite(repeat) || (repeat ?? 0) < 1) {
    return 1
  }
  return Math.floor(repeat as number)
}

export function rotationStepsToDegrees(steps: number) {
  const normalized = ((Math.floor(steps) % 4) + 4) % 4
  return normalized * 90
}

export function timelineOutputDuration(
  segments: Array<{ start: number; end: number; repeat?: number }>,
) {
  return segments.reduce(
    (sum, seg) => sum + segmentDuration(seg.start, seg.end) * effectiveSegmentRepeat(seg.repeat),
    0,
  )
}
