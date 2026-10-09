import { Fragment } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  type DragEndEvent,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { formatDurationClock } from '../api/client'
import { useProjectStore } from '../store/projectStore'
import { segmentDuration } from '../utils/time'
import type { TimelineSegment, Watermark } from '../types'

interface SegmentsTableProps {
  segments: TimelineSegment[]
  activeSegmentId: string | null
  onRowClick: (segment: TimelineSegment) => void
  onReorder: (activeId: string, overId: string) => void
  onRepeatChange: (segmentId: string, repeat: number) => void
  onRotationChange: (segmentId: string, rotationSteps: number) => void
  onAlternateRepeatReverseChange: (segmentId: string, alternateRepeatReverse: boolean) => void
  onWatermarkEnabledChange: (segmentId: string, enabled: boolean) => void
  onWatermarkChange: (segmentId: string, patch: Partial<Watermark>) => void
  emptyMessage?: string
}

interface SortableSegmentRowProps {
  segmentId: string
  selected: boolean
  onRowClick: (segment: TimelineSegment) => void
  onRepeatChange: (segmentId: string, repeat: number) => void
  onRotationChange: (segmentId: string, rotationSteps: number) => void
  onAlternateRepeatReverseChange: (segmentId: string, alternateRepeatReverse: boolean) => void
  onWatermarkEnabledChange: (segmentId: string, enabled: boolean) => void
  onWatermarkChange: (segmentId: string, patch: Partial<Watermark>) => void
}

function numberValue(raw: string): number | null {
  const parsed = Number.parseFloat(raw)
  return Number.isFinite(parsed) ? parsed : null
}

function WatermarkFields({
  segment,
  onChange,
}: {
  segment: TimelineSegment
  onChange: (patch: Partial<Watermark>) => void
}) {
  const wm = segment.watermark
  const segDuration = segmentDuration(segment.start, segment.end)

  return (
    <div className="segments-watermark-fields">
      <label className="segments-watermark-field">
        <span>Rel. start (s)</span>
        <input
          type="number"
          min={0}
          max={segDuration}
          step={0.1}
          value={wm.start}
          onClick={(e) => e.stopPropagation()}
          onMouseDown={(e) => e.stopPropagation()}
          onChange={(e) => {
            const next = numberValue(e.target.value)
            if (next === null) return
            onChange({ start: next })
          }}
        />
      </label>
      <label className="segments-watermark-field">
        <span>Rel. duration (s)</span>
        <input
          type="number"
          min={0}
          max={segDuration}
          step={0.1}
          value={wm.duration}
          onClick={(e) => e.stopPropagation()}
          onMouseDown={(e) => e.stopPropagation()}
          onChange={(e) => {
            const next = numberValue(e.target.value)
            if (next === null) return
            onChange({ duration: next })
          }}
        />
      </label>
      <label className="segments-watermark-field segments-watermark-field-text">
        <span>Watermark text</span>
        <input
          type="text"
          value={wm.text}
          placeholder="Watermark text"
          onClick={(e) => e.stopPropagation()}
          onMouseDown={(e) => e.stopPropagation()}
          onChange={(e) => onChange({ text: e.target.value })}
        />
      </label>
    </div>
  )
}

function SortableSegmentRow({
  segmentId,
  selected,
  onRowClick,
  onRepeatChange,
  onRotationChange,
  onAlternateRepeatReverseChange,
  onWatermarkEnabledChange,
  onWatermarkChange,
}: SortableSegmentRowProps) {
  const segment = useProjectStore((state) =>
    state.segments.find((entry) => entry.id === segmentId),
  )
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: segmentId })

  if (!segment) return null

  const duration = segmentDuration(segment.start, segment.end)
  const watermarkEnabled = segment.watermark?.enabled ?? false

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <Fragment>
      <tr
        ref={setNodeRef}
        style={style}
        data-segment-id={segment.id}
        className={`${selected ? 'selected' : ''}${isDragging ? ' dragging' : ''}`}
        onClick={() => onRowClick(segment)}
      >
        <td
          className="segments-table-drag"
          aria-label="Drag to reorder"
          {...attributes}
          {...listeners}
          onClick={(e) => e.stopPropagation()}
        >
          ⋮⋮
        </td>
        <td>{formatDurationClock(segment.start)}</td>
        <td>{formatDurationClock(segment.end)}</td>
        <td>{formatDurationClock(duration)}</td>
        <td>{segment.video?.name ?? segment.videoId}</td>
        <td className="segments-table-repeat">
          <input
            type="number"
            min={1}
            step={1}
            value={segment.repeat}
            aria-label="Repeat count"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            onChange={(e) => {
              const next = Number.parseInt(e.target.value, 10)
              if (Number.isNaN(next)) return
              onRepeatChange(segment.id, next)
            }}
          />
        </td>
        <td className="segments-table-repeat">
          <input
            type="number"
            min={0}
            max={3}
            step={1}
            value={segment.rotationSteps}
            aria-label="Rotation steps (90 degrees clockwise each)"
            title="0–3 clockwise quarter turns"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            onChange={(e) => {
              const next = Number.parseInt(e.target.value, 10)
              if (Number.isNaN(next)) return
              onRotationChange(segment.id, next)
            }}
          />
        </td>
        <td className="segments-table-repeat">
          <input
            type="checkbox"
            checked={segment.alternateRepeatReverse}
            aria-label="Alternate reverse on even repeats"
            title="Alternate reverse on even repeats"
            disabled={segment.repeat < 2}
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            onChange={(e) => onAlternateRepeatReverseChange(segment.id, e.target.checked)}
          />
        </td>
        <td className="segments-table-repeat">
          <input
            type="checkbox"
            checked={watermarkEnabled}
            aria-label="Enable watermark on this segment"
            title="Enable watermark on this segment"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            onChange={(e) => onWatermarkEnabledChange(segment.id, e.target.checked)}
          />
        </td>
      </tr>
      {watermarkEnabled && (
        <tr
          className={`segments-watermark-row${selected ? ' selected' : ''}`}
          onClick={(e) => e.stopPropagation()}
        >
          <td colSpan={9}>
            <WatermarkFields
              segment={segment}
              onChange={(patch) => onWatermarkChange(segment.id, patch)}
            />
          </td>
        </tr>
      )}
    </Fragment>
  )
}

export function SegmentsTable({
  segments,
  activeSegmentId,
  onRowClick,
  onReorder,
  onRepeatChange,
  onRotationChange,
  onAlternateRepeatReverseChange,
  onWatermarkEnabledChange,
  onWatermarkChange,
  emptyMessage = 'No segments yet. Select a video and click + Segment.',
}: SegmentsTableProps) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  )

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over || active.id === over.id) return
    onReorder(String(active.id), String(over.id))
  }

  const segmentIds = segments.map((seg) => seg.id)

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={handleDragEnd}
    >
      <table className="segments-table">
        <thead>
          <tr>
            <th className="segments-table-drag-header" aria-label="Reorder" />
            <th>Start time</th>
            <th>End time</th>
            <th>Duration</th>
            <th>File name</th>
            <th>Repeat</th>
            <th>Rotate</th>
            <th title="Alternate reverse on even repeats">Alt rev</th>
            <th title="Show a text watermark over this segment">WMark</th>
          </tr>
        </thead>
        <SortableContext items={segmentIds} strategy={verticalListSortingStrategy}>
          <tbody>
            {segments.length === 0 ? (
              <tr>
                <td colSpan={9} className="muted">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              segments.map((seg) => (
                <SortableSegmentRow
                  key={seg.id}
                  segmentId={seg.id}
                  selected={seg.id === activeSegmentId}
                  onRowClick={onRowClick}
                  onRepeatChange={onRepeatChange}
                  onRotationChange={onRotationChange}
                  onAlternateRepeatReverseChange={onAlternateRepeatReverseChange}
                  onWatermarkEnabledChange={onWatermarkEnabledChange}
                  onWatermarkChange={onWatermarkChange}
                />
              ))
            )}
          </tbody>
        </SortableContext>
      </table>
    </DndContext>
  )
}
