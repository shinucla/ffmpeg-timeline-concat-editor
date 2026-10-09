# Video Cut & Concat

Local web app to browse MP4 files, order clips, trim segments with frame previews, and compile a final video with FFmpeg.

## Stack

- **Frontend:** Vite + React + TypeScript
- **Backend:** Go + FFmpeg/ffprobe

## Prerequisites

- Go 1.22+
- Node.js 18+
- FFmpeg and ffprobe on `PATH`

## Quick start

```bash
# Terminal 1 — API (default port 8000)
cd backend
VIDEO_ROOT=/path/to/your/videos go run .

# Terminal 2 — UI (port 5173)
cd frontend
npm run dev
```

Open http://localhost:5173

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `VIDEO_ROOT` | `./sample-videos` | Folder scanned for `.mp4` files (overrides saved setting) |
| `OUTPUT_DIR` | `./output` | Temp working files during processing |
| `CACHE_DIR` | `./.cache` | Frame preview cache |
| `PORT` | `8000` | API listen port |

The chosen video folder is also saved to `.local/settings.json` and restored on restart (unless `VIDEO_ROOT` is set).

## Generate sample videos

```bash
./scripts/generate-samples.sh
```

## Workflow

1. **Library** — lists MP4s from `VIDEO_ROOT`
2. **Project** — add clips, drag to reorder
3. **Trim editor** — set in/out points, frame-step handles, multiple segments per clip
4. **Watermarks** — tick **WMark** on a segment row to add a text watermark. Set its
   relative start/duration and text, drag the placeholder on the preview to position
   it and the corner handle to resize, or drag the rectangle inside the timeline
   segment bar to retime it.
5. **Export** — FFmpeg cuts each segment, burns in any watermarks (position, size,
   text and timing), then concatenates into one file in `OUTPUT_DIR`

## API

- `GET /api/videos` — list videos
- `GET /api/videos/{id}/stream` — stream for preview
- `GET /api/videos/{id}/frame?t=12.34` — frame-accurate JPEG
- `POST /api/process` — start cut + concat job
- `GET /api/jobs/{id}` — job progress
