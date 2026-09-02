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

/** Group segments by file, preserve within-file timeline order, order groups by library name sort. */
export function regroupSegmentsByLibraryOrder(
  segments: TimelineSegment[],
  videos: VideoSummary[],
): TimelineSegment[] {
  const videoRank = videoRankMap(videos)
  const sorted = segmentsInTimelineOrder(segments)

  const groups = new Map<string, TimelineSegment[]>()
  for (const seg of sorted) {
    const list = groups.get(seg.videoId) ?? []
    list.push(seg)
    groups.set(seg.videoId, list)
  }

  const videoIds = [...groups.keys()].sort((a, b) => {
    const rankA = videoRank.get(a) ?? Number.MAX_SAFE_INTEGER
    const rankB = videoRank.get(b) ?? Number.MAX_SAFE_INTEGER
    return rankA - rankB
  })

  const next: TimelineSegment[] = []
  for (const videoId of videoIds) {
    next.push(...(groups.get(videoId) ?? []))
  }
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
 * Adding to a file that already has segments: regroup that file (and all files) by library order.
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
    return regroupSegmentsByLibraryOrder([...sorted, newSegment], videos)
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
