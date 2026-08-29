import type { TimelineSegment, VideoSummary } from '../types'

function videoRankMap(videos: VideoSummary[]) {
  return new Map(videos.map((video, index) => [video.id, index]))
}

function compareSegmentsByLibraryOrder(
  a: TimelineSegment,
  b: TimelineSegment,
  videoRank: Map<string, number>,
) {
  const rankA = videoRank.get(a.videoId) ?? Number.MAX_SAFE_INTEGER
  const rankB = videoRank.get(b.videoId) ?? Number.MAX_SAFE_INTEGER
  if (rankA !== rankB) return rankA - rankB
  return a.order - b.order
}

export function sortSegmentsByLibraryOrder(
  segments: TimelineSegment[],
  videos: VideoSummary[],
): TimelineSegment[] {
  const videoRank = videoRankMap(videos)
  return [...segments]
    .sort((a, b) => compareSegmentsByLibraryOrder(a, b, videoRank))
    .map((segment, order) => ({ ...segment, order }))
}

export function insertSegmentByLibraryOrder(
  segments: TimelineSegment[],
  newSegment: TimelineSegment,
  videos: VideoSummary[],
): TimelineSegment[] {
  const videoRank = videoRankMap(videos)
  const sorted = [...segments].sort((a, b) =>
    compareSegmentsByLibraryOrder(a, b, videoRank),
  )

  let insertAt = sorted.length
  let lastSameVideoIndex = -1
  for (let i = 0; i < sorted.length; i++) {
    if (sorted[i].videoId === newSegment.videoId) {
      lastSameVideoIndex = i
    }
  }

  if (lastSameVideoIndex >= 0) {
    insertAt = lastSameVideoIndex + 1
  } else {
    const newRank = videoRank.get(newSegment.videoId) ?? Number.MAX_SAFE_INTEGER
    for (let i = 0; i < sorted.length; i++) {
      const rank = videoRank.get(sorted[i].videoId) ?? Number.MAX_SAFE_INTEGER
      if (rank > newRank) {
        insertAt = i
        break
      }
    }
  }

  const next = [
    ...sorted.slice(0, insertAt),
    newSegment,
    ...sorted.slice(insertAt),
  ]
  return next.map((segment, order) => ({ ...segment, order }))
}
