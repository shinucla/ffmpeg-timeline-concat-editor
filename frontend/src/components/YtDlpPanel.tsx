import { useEffect, useRef, useState } from 'react'
import { fetchJob, startYtdlpDownload } from '../api/client'
import { useLibraryStore } from '../store/libraryStore'
import type { Job } from '../types'

interface YtdlpSegmentRow {
  id: string
  start: string
  end: string
}

const CLOCK_RE = /^\d{2}:\d{2}:\d{2}$/

function parseClockSeconds(value: string): number | null {
  if (!CLOCK_RE.test(value)) return null
  const [h, m, s] = value.split(':').map(Number)
  if (m > 59 || s > 59) return null
  return h * 3600 + m * 60 + s
}

function isValidRange(start: string, end: string): string | null {
  const startSec = parseClockSeconds(start)
  if (startSec == null) return 'Start must be HH:MM:SS'
  const endSec = parseClockSeconds(end)
  if (endSec == null) return 'End must be HH:MM:SS'
  if (endSec <= startSec) return 'End must be after start'
  return null
}

export function YtDlpPanel() {
  const loadLibrary = useLibraryStore((s) => s.loadLibrary)
  const [url, setUrl] = useState('')
  const [segments, setSegments] = useState<YtdlpSegmentRow[]>([])
  const [dialogOpen, setDialogOpen] = useState(false)
  const [startInput, setStartInput] = useState('00:00:00')
  const [endInput, setEndInput] = useState('00:00:30')
  const [dialogError, setDialogError] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [job, setJob] = useState<Job | null>(null)
  const pollRef = useRef<number | null>(null)
  const startInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current)
    }
  }, [])

  useEffect(() => {
    if (dialogOpen) {
      startInputRef.current?.focus()
      startInputRef.current?.select()
    }
  }, [dialogOpen])

  function openDialog() {
    setDialogError(null)
    setStartInput('00:00:00')
    setEndInput('00:00:30')
    setDialogOpen(true)
  }

  function closeDialog() {
    setDialogOpen(false)
    setDialogError(null)
  }

  function handleAddSegment() {
    const validationError = isValidRange(startInput.trim(), endInput.trim())
    if (validationError) {
      setDialogError(validationError)
      return
    }
    setSegments((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        start: startInput.trim(),
        end: endInput.trim(),
      },
    ])
    closeDialog()
  }

  function removeSegment(id: string) {
    setSegments((prev) => prev.filter((seg) => seg.id !== id))
  }

  async function handleDownload() {
    if (!url.trim()) {
      setError('Enter a video URL.')
      return
    }
    if (segments.length === 0) {
      setError('Add at least one segment.')
      return
    }

    setError(null)
    setNotice(null)
    setDownloading(true)
    setJob(null)

    try {
      const { jobId } = await startYtdlpDownload(
        url.trim(),
        segments.map((seg) => ({ start: seg.start, end: seg.end })),
      )
      pollRef.current = window.setInterval(async () => {
        try {
          const status = await fetchJob(jobId)
          setJob(status)
          if (status.status === 'completed' || status.status === 'failed') {
            if (pollRef.current) window.clearInterval(pollRef.current)
            setDownloading(false)
            if (status.status === 'completed') {
              setNotice(status.output ? `Saved to ${status.output}` : 'Download complete')
              void loadLibrary()
            }
          }
        } catch {
          if (pollRef.current) window.clearInterval(pollRef.current)
          setDownloading(false)
          setError('Failed to poll download job')
        }
      }, 800)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Download failed')
      setDownloading(false)
    }
  }

  return (
    <section className="panel ytdlp-panel">
      <header className="panel-header">
        <div>
          <h2>yt-dlp download</h2>
          <p className="muted small">
            yt-dlp downloads each timestamp range directly (
            <code>--download-sections</code>
            ). No app trim/concat — multiple ranges become separate files with{' '}
            <code>-01</code>, <code>-02</code>, …
          </p>
        </div>
      </header>

      <div className="ytdlp-toolbar">
        <label className="ytdlp-field ytdlp-url-field">
          <span className="ytdlp-field-label">URL</span>
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://www.youtube.com/watch?v=…"
            disabled={downloading}
          />
        </label>
        <button type="button" className="btn" disabled={downloading} onClick={openDialog}>
          + Add segment
        </button>
        <button
          type="button"
          className="btn btn-primary"
          disabled={downloading || !url.trim() || segments.length === 0}
          onClick={() => void handleDownload()}
        >
          {downloading ? 'Downloading…' : 'Download'}
        </button>
      </div>

      <table className="segments-table ytdlp-segments-table">
        <thead>
          <tr>
            <th>#</th>
            <th>Start</th>
            <th>End</th>
            <th aria-label="Actions" />
          </tr>
        </thead>
        <tbody>
          {segments.length === 0 ? (
            <tr>
              <td colSpan={4} className="muted">
                No segments yet. Click + Add segment.
              </td>
            </tr>
          ) : (
            segments.map((seg, index) => (
              <tr key={seg.id}>
                <td>{index + 1}</td>
                <td>{seg.start}</td>
                <td>{seg.end}</td>
                <td>
                  <button
                    type="button"
                    className="btn btn-ghost btn-danger"
                    disabled={downloading}
                    onClick={() => removeSegment(seg.id)}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>

      {(error || notice || job) && (
        <div className="ytdlp-status">
          {error && <p className="error small">{error}</p>}
          {notice && <p className="success small">{notice}</p>}
          {job && (
            <>
              <div className="progress-bar">
                <div
                  className="progress-fill"
                  style={{ width: `${Math.round(job.progress * 100)}%` }}
                />
              </div>
              <p className="small">
                <strong>{job.status}</strong> — {job.message}
                {job.output && (
                  <>
                    {' '}
                    · Output: <code>{job.output}</code>
                  </>
                )}
              </p>
              {job.error && <p className="error small">{job.error}</p>}
            </>
          )}
        </div>
      )}

      {dialogOpen && (
        <div className="ytdlp-dialog-backdrop" role="presentation" onClick={closeDialog}>
          <div
            className="ytdlp-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="ytdlp-dialog-title"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id="ytdlp-dialog-title">Add segment</h3>
            <p className="muted small">Enter start and end as HH:MM:SS.</p>
            <div className="ytdlp-dialog-fields">
              <label className="ytdlp-field">
                <span className="ytdlp-field-label">Start</span>
                <input
                  ref={startInputRef}
                  type="text"
                  value={startInput}
                  onChange={(e) => setStartInput(e.target.value)}
                  placeholder="00:00:00"
                  spellCheck={false}
                />
              </label>
              <label className="ytdlp-field">
                <span className="ytdlp-field-label">End</span>
                <input
                  type="text"
                  value={endInput}
                  onChange={(e) => setEndInput(e.target.value)}
                  placeholder="00:00:30"
                  spellCheck={false}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleAddSegment()
                  }}
                />
              </label>
            </div>
            {dialogError && <p className="error small">{dialogError}</p>}
            <div className="ytdlp-dialog-actions">
              <button type="button" className="btn btn-ghost" onClick={closeDialog}>
                Cancel
              </button>
              <button type="button" className="btn btn-primary" onClick={handleAddSegment}>
                Add
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
