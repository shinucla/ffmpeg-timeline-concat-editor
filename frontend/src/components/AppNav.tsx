import { useEffect, useRef, useState } from 'react'
import { fetchVideos, saveTimeline } from '../api/client'
import { useLibraryStore } from '../store/libraryStore'
import { useProjectStore } from '../store/projectStore'
import { outputNameFromTimelineFile, parseTimelineText } from '../utils/timeline'

export function AppNav() {
  const folderInput = useLibraryStore((s) => s.folderInput)
  const setFolderInput = useLibraryStore((s) => s.setFolderInput)
  const switching = useLibraryStore((s) => s.switching)
  const folderError = useLibraryStore((s) => s.folderError)
  const applyFolder = useLibraryStore((s) => s.applyFolder)
  const browseFolder = useLibraryStore((s) => s.browseFolder)
  const loadLibrary = useLibraryStore((s) => s.loadLibrary)

  const segments = useProjectStore((s) => s.segments)
  const cacheFolder = useProjectStore((s) => s.cacheFolder)
  const outputName = useProjectStore((s) => s.outputName)
  const clearProject = useProjectStore((s) => s.clearProject)
  const loadFromTimeline = useProjectStore((s) => s.loadFromTimeline)

  const fileInputRef = useRef<HTMLInputElement>(null)
  const [folderOpen, setFolderOpen] = useState(false)
  const [loadingTimeline, setLoadingTimeline] = useState(false)
  const [savingTimeline, setSavingTimeline] = useState(false)
  const [timelineError, setTimelineError] = useState<string | null>(null)
  const [timelineNotice, setTimelineNotice] = useState<string | null>(null)

  useEffect(() => {
    void loadLibrary()
  }, [loadLibrary])

  async function handleTimelineFile(file: File) {
    setLoadingTimeline(true)
    setTimelineError(null)
    setTimelineNotice(null)
    try {
      const text = await file.text()
      const { cacheFolder, entries } = parseTimelineText(text)
      const videos = await fetchVideos()
      const outputName = outputNameFromTimelineFile(file.name)
      const { loaded, missing } = loadFromTimeline(entries, videos, outputName, cacheFolder)

      if (loaded === 0) {
        throw new Error('No matching videos found in the library for this timeline.')
      }

      const cacheNote = cacheFolder ? ` Cache folder: ${cacheFolder}.` : ''
      if (missing.length > 0) {
        setTimelineNotice(
          `Loaded ${loaded} segment${loaded !== 1 ? 's' : ''}.${cacheNote} Missing videos: ${missing.join(', ')}`,
        )
      } else {
        setTimelineNotice(
          `Loaded ${loaded} segment${loaded !== 1 ? 's' : ''} from timeline.${cacheNote}`,
        )
      }
    } catch (e) {
      setTimelineError(e instanceof Error ? e.message : 'Failed to load timeline')
    } finally {
      setLoadingTimeline(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  async function handleSaveTimeline() {
    if (segments.length === 0) return

    setSavingTimeline(true)
    setTimelineError(null)
    setTimelineNotice(null)
    try {
      const missingNames = segments.filter((seg) => !seg.video?.name)
      if (missingNames.length > 0) {
        throw new Error('Some segments are missing file names and cannot be saved.')
      }

      const { path } = await saveTimeline(outputName, segments, cacheFolder)
      setTimelineNotice(`Saved timeline to ${path}`)
    } catch (e) {
      setTimelineError(e instanceof Error ? e.message : 'Failed to save timeline')
    } finally {
      setSavingTimeline(false)
    }
  }

  return (
    <header className="app-nav">
      <div className="app-nav-inner">
        <div className="app-nav-brand">
          <h1>Video Cut &amp; Concat</h1>
        </div>

        <div className="app-nav-folder">
          <button
            type="button"
            className="app-nav-folder-toggle"
            aria-expanded={folderOpen}
            aria-controls="app-nav-folder-panel"
            onClick={() => setFolderOpen((open) => !open)}
          >
            {folderOpen ? '▼' : '▶'} Video folder
          </button>
          {!folderOpen && (
            <span className="app-nav-folder-preview muted small" title={folderInput}>
              {folderInput || 'Not set'}
            </span>
          )}
          {folderOpen && (
            <div id="app-nav-folder-panel" className="app-nav-folder-panel">
              <input
                type="text"
                className="app-nav-folder-input"
                value={folderInput}
                onChange={(e) => setFolderInput(e.target.value)}
                placeholder="/path/to/your/videos"
                disabled={switching}
              />
              <button
                type="button"
                className="btn"
                disabled={switching || !folderInput.trim()}
                onClick={() => applyFolder(folderInput.trim())}
              >
                Apply
              </button>
              <button
                type="button"
                className="btn btn-ghost"
                disabled={switching}
                onClick={() => browseFolder()}
              >
                Browse…
              </button>
            </div>
          )}
        </div>

        <div className="app-nav-actions">
          <button
            type="button"
            className="btn btn-ghost"
            disabled={loadingTimeline}
            onClick={() => fileInputRef.current?.click()}
          >
            {loadingTimeline ? 'Loading…' : 'Load timeline…'}
          </button>
          <button
            type="button"
            className="btn btn-ghost"
            disabled={savingTimeline || segments.length === 0}
            onClick={() => void handleSaveTimeline()}
          >
            {savingTimeline ? 'Saving…' : 'Save timeline'}
          </button>
          <button
            type="button"
            className="btn btn-ghost"
            disabled={segments.length === 0}
            onClick={clearProject}
          >
            Reset project
          </button>
        </div>
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept=".txt,text/plain"
        hidden
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) void handleTimelineFile(file)
        }}
      />

      {(folderError || timelineError || timelineNotice || switching) && (
        <div className="app-nav-messages">
          {switching && <p className="muted small">Updating video folder…</p>}
          {folderError && <p className="error small">{folderError}</p>}
          {timelineError && <p className="error small">{timelineError}</p>}
          {timelineNotice && <p className="success small">{timelineNotice}</p>}
        </div>
      )}
    </header>
  )
}
