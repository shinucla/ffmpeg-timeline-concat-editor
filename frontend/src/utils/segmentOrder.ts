import type { TimelineSegment, VideoSummary } from '../types'

export function sortedVideosByName(videos: VideoSummary[]): VideoSummary[] {
  return [...videos].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }),
  )
}

function videoRankMap(videos: VideoSummary[]) {
  return new Map(sortedVideosByName(videos).map((video, index) => [video.id, index]))
}

export function segmentsInTimelineOrder(segments: TimelineSegment[]): TimelineSegment[] {
  return [...segments].sort((a, b) => a.order - b.order)
}

export function reassignSegmentOrder(segments: TimelineSegment[]): TimelineSegment[] {
  return segments.map((segment, order) => ({ ...segment, order }))
}

/**
 * Collect segments for one file in current timeline table order (top to bottom).
 * When rows are scattered among other files, this preserves their relative sequence.
 */
function segmentsForFileInTimelineOrder(
  sorted: TimelineSegment[],
  videoId: string,
): TimelineSegment[] {
  return sorted.filter((seg) => seg.videoId === videoId)
}

function libraryInsertIndex(
  others: TimelineSegment[],
  videoId: string,
  videoRank: Map<string, number>,
): number {
  const rank = videoRank.get(videoId) ?? Number.MAX_SAFE_INTEGER
  for (let i = 0; i < others.length; i++) {
    const otherRank = videoRank.get(others[i].videoId) ?? Number.MAX_SAFE_INTEGER
    if (otherRank > rank) {
      return i
    }
  }
  return others.length
}

/**
 * Move one file's rows into a contiguous block at the library-order slot.
 * Scattered rows for the same file are gathered in their existing table sequence;
 * optional segment is appended to the bottom of that block. Other files are untouched.
 */
export function repositionFileGroup(
  segments: TimelineSegment[],
  videoId: string,
  videos: VideoSummary[],
  appendSegment?: TimelineSegment,
): TimelineSegment[] {
  const videoRank = videoRankMap(videos)
  const sorted = segmentsInTimelineOrder(segments)

  let fileSegs = segmentsForFileInTimelineOrder(sorted, videoId)
  if (appendSegment) {
    fileSegs = [...fileSegs, appendSegment]
  }
  const others = sorted.filter((seg) => seg.videoId !== videoId)

  const insertAt = libraryInsertIndex(others, videoId, videoRank)
  const next = [
    ...others.slice(0, insertAt),
    ...fileSegs,
    ...others.slice(insertAt),
  ]
  return reassignSegmentOrder(next)
}

/** Insert a segment for a file that is not yet on the timeline; leave other mixed groups unchanged. */
export function insertSegmentForNewFile(
  segments: TimelineSegment[],
  newSegment: TimelineSegment,
  videos: VideoSummary[],
): TimelineSegment[] {
  const videoRank = videoRankMap(videos)
  const sorted = segmentsInTimelineOrder(segments)
  const newRank = videoRank.get(newSegment.videoId) ?? Number.MAX_SAFE_INTEGER

  let insertAt = sorted.length
  for (let i = 0; i < sorted.length; i++) {
    const rank = videoRank.get(sorted[i].videoId) ?? Number.MAX_SAFE_INTEGER
    if (rank > newRank) {
      insertAt = i
      break
    }
  }

  const next = [...sorted.slice(0, insertAt), newSegment, ...sorted.slice(insertAt)]
  return reassignSegmentOrder(next)
}

/**
 * Adding to a file that already has segments: reposition that file group to library order,
 * append the new segment at the bottom of the group, and preserve within-group order.
 * Adding to a new file: insert at library position without regrouping existing mixed files.
 */
export function insertSegmentByLibraryOrder(
  segments: TimelineSegment[],
  newSegment: TimelineSegment,
  videos: VideoSummary[],
): TimelineSegment[] {
  const sorted = segmentsInTimelineOrder(segments)
  const hasExistingForFile = sorted.some((seg) => seg.videoId === newSegment.videoId)

  if (hasExistingForFile) {
    return repositionFileGroup(sorted, newSegment.videoId, videos, newSegment)
  }
  return insertSegmentForNewFile(sorted, newSegment, videos)
}

export function moveSegmentInTimelineOrder(
  segments: TimelineSegment[],
  activeId: string,
  overId: string,
): TimelineSegment[] {
  const sorted = segmentsInTimelineOrder(segments)
  const fromIndex = sorted.findIndex((seg) => seg.id === activeId)
  const toIndex = sorted.findIndex((seg) => seg.id === overId)
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return segments
  }
  const next = [...sorted]
  const [moved] = next.splice(fromIndex, 1)
  next.splice(toIndex, 0, moved)
  return reassignSegmentOrder(next)
}
