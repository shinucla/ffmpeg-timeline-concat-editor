import { useMemo, useState } from 'react'
import { formatDuration, formatSize } from '../api/client'
import { useLibraryStore } from '../store/libraryStore'
import { useProjectStore } from '../store/projectStore'
import type { VideoSort, VideoSummary } from '../types'

function sortVideos(videos: VideoSummary[], sort: VideoSort): VideoSummary[] {
  const next = [...videos]
  next.sort((a, b) => {
    switch (sort) {
      case 'name-asc':
        return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
      case 'name-desc':
        return b.name.localeCompare(a.name, undefined, { sensitivity: 'base' })
      case 'size-asc':
        return a.size - b.size
      case 'size-desc':
        return b.size - a.size
      case 'modified-asc':
        return a.modifiedAt - b.modifiedAt
      case 'modified-desc':
        return b.modifiedAt - a.modifiedAt
      default:
        return 0
    }
  })
  return next
}

export function Library() {
  const videos = useLibraryStore((s) => s.videos)
  const loading = useLibraryStore((s) => s.loading)
  const switching = useLibraryStore((s) => s.switching)
  const selectedVideoId = useProjectStore((s) => s.selectedVideoId)
  const selectVideo = useProjectStore((s) => s.selectVideo)
  const segments = useProjectStore((s) => s.segments)
  const [sort, setSort] = useState<VideoSort>('name-asc')

  const sortedVideos = useMemo(() => sortVideos(videos, sort), [videos, sort])
  const segmentCountsByVideo = useMemo(() => {
    const counts = new Map<string, number>()
    for (const seg of segments) {
      counts.set(seg.videoId, (counts.get(seg.videoId) ?? 0) + 1)
    }
    return counts
  }, [segments])

  function handleSelect(video: VideoSummary) {
    selectVideo(video.id)
  }

  if (loading) return <div className="panel muted">Loading videos…</div>

  return (
    <section className="panel panel-library">
      <header className="panel-header">
        <div className="panel-header-left">
          <h2>Library</h2>
          <span className="badge">{videos.length} files</span>
        </div>
      </header>

      {videos.length > 0 && (
        <div className="field-row sort-field">
          <span className="field-row-label">Sort by</span>
          <select
            className="field-row-control"
            value={sort}
            onChange={(e) => setSort(e.target.value as VideoSort)}
            disabled={switching}
          >
            <option value="name-asc">Name (A → Z)</option>
            <option value="name-desc">Name (Z → A)</option>
            <option value="size-asc">Size (smallest first)</option>
            <option value="size-desc">Size (largest first)</option>
            <option value="modified-asc">Modified (oldest first)</option>
            <option value="modified-desc">Modified (newest first)</option>
          </select>
        </div>
      )}

      {videos.length === 0 ? (
        <p className="muted">
          No MP4 files found. Choose a video folder from the top bar.
        </p>
      ) : (
        <ul className="video-list video-list-scroll">
          {sortedVideos.map((video) => {
            const segCount = segmentCountsByVideo.get(video.id) ?? 0
            const selected = video.id === selectedVideoId
            return (
              <li
                key={video.id}
                className={`video-card video-card-selectable ${selected ? 'selected' : ''}`}
                onClick={() => handleSelect(video)}
              >
                <div className="video-card-main">
                  <div className="video-card-title-row">
                    <strong className="video-filename">{video.name}</strong>
                    {segCount > 0 && (
                      <span className="video-segment-indicator" title={`${segCount} segment(s)`}>
                        {segCount}
                      </span>
                    )}
                  </div>
                  <span className="muted small">
                    {formatDuration(video.duration)} · {video.width}×{video.height} ·{' '}
                    {formatSize(video.size)}
                  </span>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
