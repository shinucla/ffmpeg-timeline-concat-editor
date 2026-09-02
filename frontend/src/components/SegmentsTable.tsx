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
import type { TimelineSegment } from '../types'

interface SegmentsTableProps {
  segments: TimelineSegment[]
  activeSegmentId: string | null
  onRowClick: (segment: TimelineSegment) => void
  onReorder: (activeId: string, overId: string) => void
  onRepeatChange: (segmentId: string, repeat: number) => void
  onRotationChange: (segmentId: string, rotationSteps: number) => void
  onAlternateRepeatReverseChange: (segmentId: string, alternateRepeatReverse: boolean) => void
  emptyMessage?: string
}

interface SortableSegmentRowProps {
  segmentId: string
  selected: boolean
  onRowClick: (segment: TimelineSegment) => void
  onRepeatChange: (segmentId: string, repeat: number) => void
  onRotationChange: (segmentId: string, rotationSteps: number) => void
  onAlternateRepeatReverseChange: (segmentId: string, alternateRepeatReverse: boolean) => void
}

function SortableSegmentRow({
  segmentId,
  selected,
  onRowClick,
  onRepeatChange,
  onRotationChange,
  onAlternateRepeatReverseChange,
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

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
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
    </tr>
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
          </tr>
        </thead>
        <SortableContext items={segmentIds} strategy={verticalListSortingStrategy}>
          <tbody>
            {segments.length === 0 ? (
              <tr>
                <td colSpan={8} className="muted">
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
                />
              ))
            )}
          </tbody>
        </SortableContext>
      </table>
    </DndContext>
  )
}
