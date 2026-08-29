import { useEffect, useRef, useState } from 'react'
import { fetchJob, formatDurationClock, startProcess } from '../api/client'
import { useProjectStore } from '../store/projectStore'
import type { Job } from '../types'

export function ProcessPanel() {
  const segments = useProjectStore((s) => s.segments)
  const cacheFolder = useProjectStore((s) => s.cacheFolder)
  const outputName = useProjectStore((s) => s.outputName)
  const setOutputName = useProjectStore((s) => s.setOutputName)
  const setCacheFolder = useProjectStore((s) => s.setCacheFolder)
  const [job, setJob] = useState<Job | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [processing, setProcessing] = useState(false)
  const [forceRecut, setForceRecut] = useState(false)
  const pollRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current)
    }
  }, [])

  async function handleProcess() {
    if (segments.length === 0) {
      setError('Add at least one segment to the timeline.')
      return
    }
    setError(null)
    setProcessing(true)
    setJob(null)
    try {
      const { jobId } = await startProcess(
        outputName,
        segments,
        forceRecut ? null : cacheFolder,
        forceRecut,
      )
      pollRef.current = window.setInterval(async () => {
        try {
          const status = await fetchJob(jobId)
          setJob(status)
          if (status.status === 'completed' || status.status === 'failed') {
            if (pollRef.current) window.clearInterval(pollRef.current)
            setProcessing(false)
            if (status.status === 'completed' && status.cacheFolder) {
              setCacheFolder(status.cacheFolder)
            }
          }
        } catch {
          if (pollRef.current) window.clearInterval(pollRef.current)
          setProcessing(false)
        }
      }, 800)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Process failed')
      setProcessing(false)
    }
  }

  const totalDuration = segments.reduce(
    (sum, seg) => sum + Math.max(0, seg.end - seg.start) * seg.repeat,
    0,
  )
  const uniqueVideos = new Set(segments.map((seg) => seg.videoId)).size

  return (
    <footer className="export-bar">
      <div className="export-bar-inner">
        <div className="export-bar-summary">
          <strong className="export-bar-title">Export</strong>
          <span className="badge">
            {segments.length} segments · {formatDurationClock(totalDuration)}
          </span>
          <span className="muted small">
            {uniqueVideos} file{uniqueVideos !== 1 ? 's' : ''}
          </span>
        </div>

        <label className="export-bar-field">
          <span className="export-bar-field-label">Output filename</span>
          <input
            type="text"
            value={outputName}
            onChange={(e) => setOutputName(e.target.value)}
            placeholder="final-output"
          />
        </label>

        <label className="export-bar-option">
          <input
            type="checkbox"
            checked={forceRecut}
            disabled={processing}
            onChange={(e) => setForceRecut(e.target.checked)}
          />
          <span>Re-cut all segments</span>
        </label>

        <button
          type="button"
          className="btn btn-primary export-bar-process"
          disabled={processing || segments.length === 0}
          onClick={handleProcess}
        >
          {processing ? 'Processing…' : 'Process'}
        </button>

        {(error || job) && (
          <div className="export-bar-status">
            {error && <p className="error small">{error}</p>}
            {job && (
              <>
                <div className="progress-bar">
                  <div
                    className="progress-fill"
                    style={{ width: `${Math.round(job.progress * 100)}%` }}
                  />
                </div>
                <p className="small export-bar-job-message">
                  <strong>{job.status}</strong> — {job.message}
                  {job.output && (
                    <>
                      {' '}
                      · Output: <code>{job.output}</code>
                    </>
                  )}
                  {job.manifest && (
                    <>
                      {' '}
                      · Timeline: <code>{job.manifest}</code>
                    </>
                  )}
                </p>
                {job.error && <p className="error small">{job.error}</p>}
              </>
            )}
          </div>
        )}
      </div>
    </footer>
  )
}
