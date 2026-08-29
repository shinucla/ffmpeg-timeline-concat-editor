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
