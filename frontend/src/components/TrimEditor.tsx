import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { formatDuration, formatSegmentBarDuration, streamUrl } from '../api/client'
import { SegmentsTable } from './SegmentsTable'
import { sortSegmentsByLibraryOrder } from '../utils/segmentOrder'
import { frameStep, quantizeToFrame, segmentDuration } from '../utils/time'
import {
  applySegmentDragUpdate,
  computeNewSegmentRange,
  getOtherSegments,
} from '../utils/segmentBounds'
import { useLibraryStore } from '../store/libraryStore'
import { usePlayerStore } from '../store/playerStore'
import { useProjectStore } from '../store/projectStore'
import type { TimelineSegment, VideoSummary } from '../types'

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function segmentContains(seg: TimelineSegment, time: number) {
  return time >= seg.start && time < seg.end
}

function findSegmentAtTime(segs: TimelineSegment[], time: number) {
  return segs.find((seg) => segmentContains(seg, time))
}

function nextSegmentStartWhenOutside(segs: TimelineSegment[], time: number) {
  const ordered = [...segs].sort((a, b) => a.order - b.order)
  if (ordered.length === 0) return 0
  const upcoming = ordered.find((seg) => seg.start > time)
  return upcoming?.start ?? ordered[0].start
}

const TIMELINE_ZOOM_LEVELS = [1, 2, 4, 8, 16, 32, 64] as const

type DragMode = 'resize-start' | 'resize-end' | 'move' | 'playhead' | null

const SEGMENT_REPEAT_HEADROOM = 32
const SEGMENT_BAR_TOP = 8 + SEGMENT_REPEAT_HEADROOM
const SEGMENT_BAR_HEIGHT = 40
const TIMELINE_BASE_HEIGHT = SEGMENT_BAR_TOP + SEGMENT_BAR_HEIGHT + 8
const LINK_LANE_GAP = 8
const LINK_TIME_OVERLAP_EPS = 1e-6

function linkLanesHeight(maxLaneIndex: number) {
  if (maxLaneIndex < 0) return 0
  return 6 + (maxLaneIndex + 1) * LINK_LANE_GAP
}

function linkTimeInterval(from: TimelineSegment, to: TimelineSegment) {
  return {
    left: Math.min(from.end, to.start),
    right: Math.max(from.end, to.start),
  }
}

function linkTimeRangesOverlap(
  a: { left: number; right: number },
  b: { left: number; right: number },
) {
  const overlapStart = Math.max(a.left, b.left)
  const overlapEnd = Math.min(a.right, b.right)
  return overlapEnd - overlapStart > LINK_TIME_OVERLAP_EPS
}

function assignSegmentLinkLanes(
  links: Array<{ from: TimelineSegment; to: TimelineSegment }>,
) {
  const lanes: Array<Array<{ left: number; right: number }>> = []
  const assignments: number[] = []

  for (const link of links) {
    const interval = linkTimeInterval(link.from, link.to)
    let lane = 0
    for (; lane < lanes.length; lane++) {
      const blocked = lanes[lane].some((existing) =>
        linkTimeRangesOverlap(interval, existing),
      )
      if (!blocked) break
    }
    if (lane === lanes.length) lanes.push([])
    lanes[lane].push(interval)
    assignments.push(lane)
  }

  return assignments
}

function segmentLinkPath(
  from: TimelineSegment,
  to: TimelineSegment,
  duration: number,
  lane: number,
): string {
  const yBar = SEGMENT_BAR_TOP + SEGMENT_BAR_HEIGHT / 2
  const yDrop = SEGMENT_BAR_TOP + SEGMENT_BAR_HEIGHT + 2
  const laneY = TIMELINE_BASE_HEIGHT + 4 + lane * LINK_LANE_GAP
  const x1 = (from.end / duration) * 100
  const x2 = (to.start / duration) * 100

  return `M ${x1} ${yBar} L ${x1} ${yDrop} L ${x1} ${laneY} L ${x2} ${laneY} L ${x2} ${yDrop} L ${x2} ${yBar}`
}

function segmentLinkLabelPoint(
  from: TimelineSegment,
  to: TimelineSegment,
  duration: number,
  lane: number,
  order: number,
) {
  const laneY = TIMELINE_BASE_HEIGHT + 4 + lane * LINK_LANE_GAP
  const x1 = (from.end / duration) * 100
  const x2 = (to.start / duration) * 100
  return { x: (x1 + x2) / 2, y: laneY, order }
}

export function TrimEditor() {
  const videos = useLibraryStore((s) => s.videos)
  const segments = useProjectStore((s) => s.segments)
  const selectedVideoId = useProjectStore((s) => s.selectedVideoId)
  const activeSegmentId = useProjectStore((s) => s.activeSegmentId)
  const selectSegment = useProjectStore((s) => s.selectSegment)
  const addSegment = useProjectStore((s) => s.addSegment)
  const updateSegment = useProjectStore((s) => s.updateSegment)
  const adjustSegmentRepeat = useProjectStore((s) => s.adjustSegmentRepeat)
  const reorderSegments = useProjectStore((s) => s.reorderSegments)
  const setSegmentRepeat = useProjectStore((s) => s.setSegmentRepeat)
  const setSegmentRotationSteps = useProjectStore((s) => s.setSegmentRotationSteps)
  const removeSegment = useProjectStore((s) => s.removeSegment)
  const videoRoot = useLibraryStore((s) => s.folderInput)
  const applyPlayerSettings = usePlayerStore((s) => s.applyTo)
  const syncPlayerSettings = usePlayerStore((s) => s.syncFrom)

  const video = useMemo(
    () => videos.find((v) => v.id === selectedVideoId),
    [videos, selectedVideoId],
  )

  const videoSegments = useMemo(
    () =>
      segments
        .filter((seg) => seg.videoId === selectedVideoId)
        .sort((a, b) => a.start - b.start),
    [segments, selectedVideoId],
  )

  const videoSegmentsByOrder = useMemo(
    () =>
      segments
        .filter((seg) => seg.videoId === selectedVideoId)
        .sort((a, b) => a.order - b.order),
    [segments, selectedVideoId],
  )

  const sortedSegments = useMemo(
    () => sortSegmentsByLibraryOrder(segments, videos),
    [segments, videos],
  )

  const videoRef = useRef<HTMLVideoElement>(null)
  const trackRef = useRef<HTMLDivElement>(null)
  const viewportRef = useRef<HTMLDivElement>(null)
  const [currentTime, setCurrentTime] = useState(0)
  const [timelineZoomIndex, setTimelineZoomIndex] = useState(0)
  const dragRef = useRef<{
    mode: DragMode
    segmentId: string
    startX: number
    origin: { start: number; end: number }
  } | null>(null)
  const scrubbingRef = useRef(false)
  const prevPlaybackTimeRef = useRef(0)
  const currentTimeRef = useRef(0)
  const segmentRepeatProgressRef = useRef<Map<string, number>>(new Map())
  const programmaticSeekRef = useRef(false)
  const repeatSeekPendingRef = useRef<{ start: number; end: number } | null>(null)

  const duration = video?.duration ?? 0
  const fps = video?.fps ?? 30
  const minDuration = Math.max(0.1, frameStep(fps))
  const timelineZoom = TIMELINE_ZOOM_LEVELS[timelineZoomIndex]

  function scrollTimelineToPlayhead(time = currentTimeRef.current) {
    const viewport = viewportRef.current
    const track = trackRef.current
    if (!viewport || !track || duration <= 0) return

    const playheadX = (time / duration) * track.offsetWidth
    const margin = 48
    const viewLeft = viewport.scrollLeft
    const viewRight = viewLeft + viewport.clientWidth

    if (playheadX < viewLeft + margin) {
      viewport.scrollLeft = Math.max(0, playheadX - margin)
    } else if (playheadX > viewRight - margin) {
      viewport.scrollLeft = Math.max(0, playheadX - viewport.clientWidth + margin)
    }
  }

  function setTimelineZoomIndexClamped(nextIndex: number) {
    setTimelineZoomIndex(
      clamp(nextIndex, 0, TIMELINE_ZOOM_LEVELS.length - 1),
    )
  }

  function zoomTimeline(direction: 1 | -1) {
    setTimelineZoomIndexClamped(timelineZoomIndex + direction)
  }

  function resetTimelineZoom() {
    setTimelineZoomIndex(0)
  }

  function handleTimelineWheel(e: React.WheelEvent) {
    if (!e.ctrlKey && !e.metaKey) return
    e.preventDefault()
    zoomTimeline(e.deltaY > 0 ? -1 : 1)
  }

  function seekToTime(time: number, programmatic = false) {
    const bounded = duration > 0 ? clamp(time, 0, duration) : Math.max(0, time)
    const quantized = quantizeToFrame(bounded, fps)
    currentTimeRef.current = quantized
    setCurrentTime(quantized)
    prevPlaybackTimeRef.current = quantized
    if (videoRef.current) {
      // Mark before seeking so the resulting `seeked` event is not re-handled.
      programmaticSeekRef.current = true
      videoRef.current.currentTime = quantized
    }
    if (!programmatic) {
      requestAnimationFrame(() => scrollTimelineToPlayhead(quantized))
    }
  }

  useEffect(() => {
    if (!video) return
    setTimelineZoomIndex(0)
    const { activeSegmentId: activeId, segments: segs } = useProjectStore.getState()
    const fileSegments = segs
      .filter((s) => s.videoId === video.id)
      .sort((a, b) => a.order - b.order)
    const active = fileSegments.find((s) => s.id === activeId)
    if (active) {
      seekToTime(active.start, true)
    } else if (fileSegments.length > 0) {
      seekToTime(fileSegments[0].start, true)
    } else {
      seekToTime(0, true)
    }
  }, [video?.id])

  useEffect(() => {
    requestAnimationFrame(() => scrollTimelineToPlayhead())
  }, [timelineZoomIndex, duration])

  function handleTableSegmentClick(seg: { id: string; videoId: string; start: number }) {
    selectSegment(seg.id)
    if (seg.videoId === selectedVideoId) {
      seekToTime(seg.start)
    }
  }

  function seekFromClientX(clientX: number) {
    const track = trackRef.current
    if (!track || duration <= 0) return
    const rect = track.getBoundingClientRect()
    const ratio = clamp((clientX - rect.left) / rect.width, 0, 1)
    seekToTime(ratio * duration)
    videoRef.current?.pause()
  }

  useEffect(() => {
    function onMouseMove(e: MouseEvent) {
      const drag = dragRef.current
      const track = trackRef.current
      if (!drag || !track || !video || duration <= 0) return

      if (drag.mode === 'playhead') {
        seekFromClientX(e.clientX)
        return
      }

      const rect = track.getBoundingClientRect()
      const deltaRatio = (e.clientX - drag.startX) / rect.width
      const delta = deltaRatio * duration
      const playhead = quantizeToFrame(currentTimeRef.current, fps)

      if (!drag.mode) return

      const fileSegs = useProjectStore
        .getState()
        .segments.filter((seg) => seg.videoId === video.id)
      const others = getOtherSegments(drag.segmentId, fileSegs)

      const nextRange = applySegmentDragUpdate(
        drag.mode,
        drag.origin,
        delta,
        playhead,
        rect.width,
        duration,
        minDuration,
        fps,
        others,
      )
      if (!nextRange) return

      updateSegment(drag.segmentId, nextRange.start, nextRange.end)
    }

    function onMouseUp() {
      if (dragRef.current?.mode === 'playhead') {
        scrubbingRef.current = false
      }
      dragRef.current = null
    }

    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
    return () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
  }, [video, duration, minDuration, fps, updateSegment])

  const segmentLinkMarkerId = `segment-link-arrow-${useId().replace(/:/g, '')}`

  const segmentLinks = useMemo(() => {
    if (duration <= 0 || videoSegmentsByOrder.length < 2) return []

    const pairs = videoSegmentsByOrder.slice(0, -1).map((seg, index) => ({
      key: `${seg.id}-${videoSegmentsByOrder[index + 1].id}`,
      from: seg,
      to: videoSegmentsByOrder[index + 1],
      index,
    }))
    const lanes = assignSegmentLinkLanes(pairs)

    return pairs.map((link, i) => ({
      ...link,
      lane: lanes[i],
    }))
  }, [duration, videoSegmentsByOrder])

  const maxLinkLane = segmentLinks.reduce((max, link) => Math.max(max, link.lane), -1)
  const timelineTrackHeight = TIMELINE_BASE_HEIGHT + linkLanesHeight(maxLinkLane)

  if (!video) {
    return (
      <section className="panel trim-editor">
        {videoRoot && (
          <p className="preview-source muted small">
            Source: <code>{videoRoot}</code>
          </p>
        )}
        <h2>Editor</h2>
        <p className="muted">Select a video from the library to preview and trim.</p>

        {sortedSegments.length > 0 && (
          <SegmentsTable
            segments={sortedSegments}
            activeSegmentId={activeSegmentId}
            onRowClick={handleTableSegmentClick}
            onReorder={reorderSegments}
            onRepeatChange={setSegmentRepeat}
            onRotationChange={setSegmentRotationSteps}
            emptyMessage="No segments yet."
          />
        )}
      </section>
    )
  }

  const activeVideo: VideoSummary = video

  function handleTimelinePointerDown(e: React.MouseEvent) {
    const target = e.target as HTMLElement
    if (target.closest('.segment-remove') || target.closest('.timeline-playhead')) {
      return
    }
    if (target.closest('.segment-repeat')) {
      return
    }

    const bar = target.closest('.segment-bar') as HTMLElement | null
    const segmentId = bar?.dataset.segmentId ?? ''

    if (target.closest('.segment-handle-left') && segmentId) {
      selectSegment(segmentId)
      beginDrag(e, segmentId, 'resize-start')
      return
    }
    if (target.closest('.segment-handle-right') && segmentId) {
      selectSegment(segmentId)
      beginDrag(e, segmentId, 'resize-end')
      return
    }
    if (target.closest('.segment-body') && segmentId) {
      selectSegment(segmentId)
      beginDrag(e, segmentId, 'move')
      return
    }

    if (segmentId) {
      selectSegment(segmentId)
    }
  }

  function handlePlayheadPointerDown(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    beginScrub(e)
  }

  function handleSegmentDoubleClick(e: React.MouseEvent, segmentId: string) {
    e.preventDefault()
    e.stopPropagation()
    selectSegment(segmentId)
    const seg = segments.find((s) => s.id === segmentId)
    if (seg) seekToTime(seg.start)
  }

  function beginScrub(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    scrubbingRef.current = true
    dragRef.current = {
      mode: 'playhead',
      segmentId: '',
      startX: e.clientX,
      origin: { start: 0, end: 0 },
    }
    seekFromClientX(e.clientX)
  }

  function beginDrag(e: React.MouseEvent, segmentId: string, mode: DragMode) {
    e.preventDefault()
    e.stopPropagation()
    const seg = segments.find((s) => s.id === segmentId)
    if (!seg) return
    dragRef.current = {
      mode,
      segmentId,
      startX: e.clientX,
      origin: { start: seg.start, end: seg.end },
    }
  }

  function handleAddSegment() {
    const range = computeNewSegmentRange(
      currentTimeRef.current,
      videoSegments,
      duration,
      minDuration,
      fps,
    )
    if (!range) return
    addSegment(activeVideo, range.start, range.end)
  }

  function resetSegmentRepeatProgress() {
    segmentRepeatProgressRef.current.clear()
    repeatSeekPendingRef.current = null
  }

  function seekAfterSegmentEnd(playingSeg: TimelineSegment) {
    const video = videoRef.current
    const wasPlaying = Boolean(video && !video.paused)
    const completed = (segmentRepeatProgressRef.current.get(playingSeg.id) ?? 0) + 1

    if (completed < playingSeg.repeat) {
      segmentRepeatProgressRef.current.set(playingSeg.id, completed)
      repeatSeekPendingRef.current = { start: playingSeg.start, end: playingSeg.end }
      seekToTime(playingSeg.start, true)
      if (wasPlaying) {
        void video?.play()
      }
      return
    }

    segmentRepeatProgressRef.current.delete(playingSeg.id)
    repeatSeekPendingRef.current = null

    const ordered = videoSegmentsByOrder
    const index = ordered.findIndex((seg) => seg.id === playingSeg.id)
    const hasNext = index >= 0 && index < ordered.length - 1

    if (!hasNext) {
      video?.pause()
      return
    }

    seekToTime(ordered[index + 1].start, true)
    if (wasPlaying) {
      void video?.play()
    }
  }

  function handleVideoPlay() {
    if (videoSegmentsByOrder.length === 0) return

    const t = currentTimeRef.current
    if (findSegmentAtTime(videoSegmentsByOrder, t)) return

    resetSegmentRepeatProgress()
    seekToTime(nextSegmentStartWhenOutside(videoSegmentsByOrder, t), true)
  }

  function handleVideoTimeUpdate(t: number) {
    if (scrubbingRef.current) return

    const pendingRepeatSeek = repeatSeekPendingRef.current
    if (pendingRepeatSeek) {
      const endEpsilon = frameStep(fps) / 2
      if (t >= pendingRepeatSeek.end - endEpsilon) {
        return
      }
      repeatSeekPendingRef.current = null
    }

    const prev = prevPlaybackTimeRef.current
    prevPlaybackTimeRef.current = t
    currentTimeRef.current = t
    setCurrentTime(t)

    if (!videoRef.current || videoRef.current.paused) return
    if (videoSegmentsByOrder.length === 0) return

    const endEpsilon = frameStep(fps) / 2
    const playingSeg =
      findSegmentAtTime(videoSegmentsByOrder, prev) ??
      findSegmentAtTime(videoSegmentsByOrder, prev - endEpsilon)
    if (!playingSeg) return

    const crossedEndNaturally =
      prev < playingSeg.end - endEpsilon &&
      t >= playingSeg.end - endEpsilon &&
      t - prev < 1

    if (crossedEndNaturally) {
      seekAfterSegmentEnd(playingSeg)
    }
  }

  function handleVideoSeeked(t: number) {
    if (programmaticSeekRef.current) {
      programmaticSeekRef.current = false
      const pendingRepeatSeek = repeatSeekPendingRef.current
      if (
        pendingRepeatSeek &&
        Math.abs(t - pendingRepeatSeek.start) <= frameStep(fps) * 2
      ) {
        repeatSeekPendingRef.current = null
        prevPlaybackTimeRef.current = t
        currentTimeRef.current = t
        setCurrentTime(t)
      }
      return
    }
    resetSegmentRepeatProgress()
    seekToTime(t)
  }

  return (
    <section className="panel trim-editor">
      {videoRoot && (
        <p className="preview-source muted small">
          Source: <code>{videoRoot}</code>
        </p>
      )}

      <header className="panel-header">
        <div>
          <h2>{activeVideo.name}</h2>
          <p className="muted small">
            {formatDuration(duration)} total · {fps.toFixed(2)} fps
          </p>
        </div>
      </header>

      <video
        ref={videoRef}
        className="preview-video"
        src={streamUrl(activeVideo.id)}
        controls
        onLoadedMetadata={(e) => applyPlayerSettings(e.currentTarget)}
        onVolumeChange={(e) => syncPlayerSettings(e.currentTarget)}
        onRateChange={(e) => syncPlayerSettings(e.currentTarget)}
        onPlay={handleVideoPlay}
        onTimeUpdate={(e) => handleVideoTimeUpdate(e.currentTarget.currentTime)}
        onSeeked={(e) => handleVideoSeeked(e.currentTarget.currentTime)}
      />

      <div
        className="timeline-viewport"
        ref={viewportRef}
        onWheel={handleTimelineWheel}
      >
        <div
          className="timeline-track"
          ref={trackRef}
          style={{ width: `${timelineZoom * 100}%`, height: timelineTrackHeight }}
          onMouseDown={handleTimelinePointerDown}
        >
        {segmentLinks.length > 0 && (
          <svg
            className="timeline-segment-links"
            viewBox={`0 0 100 ${timelineTrackHeight}`}
            preserveAspectRatio="none"
            aria-hidden
          >
            <defs>
              <marker
                id={segmentLinkMarkerId}
                markerWidth="6"
                markerHeight="6"
                refX="5"
                refY="3"
                orient="auto"
                markerUnits="strokeWidth"
              >
                <path d="M0,0 L6,3 L0,6 Z" className="timeline-segment-link-head" />
              </marker>
            </defs>
            {segmentLinks.map((link) => (
              <path
                key={link.key}
                className="timeline-segment-link"
                data-lane={link.lane}
                d={segmentLinkPath(link.from, link.to, duration, link.lane)}
                markerEnd={`url(#${segmentLinkMarkerId})`}
              />
            ))}
          </svg>
        )}
        {segmentLinks.map((link) => {
          const label = segmentLinkLabelPoint(
            link.from,
            link.to,
            duration,
            link.lane,
            link.index + 1,
          )
          return (
            <span
              key={`${link.key}-label`}
              className="timeline-segment-link-label"
              style={{
                left: `${label.x}%`,
                top: `${(label.y / timelineTrackHeight) * 100}%`,
              }}
            >
              {label.order}
            </span>
          )
        })}
        {videoSegments.map((seg) => {
          const left = duration > 0 ? (seg.start / duration) * 100 : 0
          const width = duration > 0 ? ((seg.end - seg.start) / duration) * 100 : 0
          const segDuration = segmentDuration(seg.start, seg.end)
          const active = seg.id === activeSegmentId
          return (
            <div
              key={seg.id}
              data-segment-id={seg.id}
              className={`segment-bar ${active ? 'active' : ''}`}
              style={{ left: `${left}%`, width: `${width}%` }}
              onDoubleClick={(e) => handleSegmentDoubleClick(e, seg.id)}
            >
              <span className="segment-duration-label">
                {formatSegmentBarDuration(segDuration)}
              </span>
              <div
                className="segment-repeat"
                onMouseDown={(e) => {
                  e.stopPropagation()
                  selectSegment(seg.id)
                }}
                onDoubleClick={(e) => e.stopPropagation()}
              >
                <div className="segment-repeat-controls">
                  <button
                    type="button"
                    className="segment-repeat-btn"
                    aria-label="Decrease repeat count"
                    disabled={seg.repeat <= 1}
                    onClick={(e) => {
                      e.stopPropagation()
                      adjustSegmentRepeat(seg.id, -1)
                    }}
                  >
                    −
                  </button>
                  <button
                    type="button"
                    className="segment-repeat-btn"
                    aria-label="Increase repeat count"
                    onClick={(e) => {
                      e.stopPropagation()
                      adjustSegmentRepeat(seg.id, 1)
                    }}
                  >
                    +
                  </button>
                </div>
                <span className="segment-repeat-badge">{seg.repeat}</span>
              </div>
              <button
                type="button"
                className="segment-remove"
                aria-label="Remove segment"
                onClick={(e) => {
                  e.stopPropagation()
                  removeSegment(seg.id)
                }}
              >
                ×
              </button>
              <div className="segment-handle segment-handle-left" />
              <div className="segment-body" />
              <div className="segment-handle segment-handle-right" />
            </div>
          )
        })}
        <div
          className="timeline-playhead"
          style={{
            left: `${duration > 0 ? (currentTime / duration) * 100 : 0}%`,
            height: timelineTrackHeight,
          }}
          role="slider"
          aria-label="Playhead"
          aria-valuemin={0}
          aria-valuemax={duration}
          aria-valuenow={currentTime}
          onMouseDown={handlePlayheadPointerDown}
        />
        </div>
      </div>

      <div className="timeline-actions">
        <button type="button" className="btn" onClick={handleAddSegment}>
          + Segment
        </button>
        <div className="timeline-zoom" aria-label="Timeline zoom">
          <button
            type="button"
            className="btn btn-ghost timeline-zoom-btn"
            aria-label="Zoom out"
            disabled={timelineZoomIndex === 0}
            onClick={() => zoomTimeline(-1)}
          >
            −
          </button>
          <span className="timeline-zoom-label">{timelineZoom}×</span>
          <button
            type="button"
            className="btn btn-ghost timeline-zoom-btn"
            aria-label="Zoom in"
            disabled={timelineZoomIndex >= TIMELINE_ZOOM_LEVELS.length - 1}
            onClick={() => zoomTimeline(1)}
          >
            +
          </button>
          <button
            type="button"
            className="btn btn-ghost timeline-zoom-fit"
            disabled={timelineZoomIndex === 0}
            onClick={resetTimelineZoom}
          >
            Fit
          </button>
          <span className="muted small timeline-zoom-hint">Ctrl+scroll to zoom</span>
        </div>
      </div>

      <SegmentsTable
        segments={sortedSegments}
        activeSegmentId={activeSegmentId}
        onRowClick={handleTableSegmentClick}
        onReorder={reorderSegments}
        onRepeatChange={setSegmentRepeat}
        onRotationChange={setSegmentRotationSteps}
      />
    </section>
  )
}
