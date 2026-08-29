package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shin/web-load-time-cut-concat/backend/config"
	"github.com/shin/web-load-time-cut-concat/backend/ffmpeg"
	"github.com/shin/web-load-time-cut-concat/backend/models"
	"github.com/shin/web-load-time-cut-concat/backend/store"
)

type Handler struct {
	cfg  *config.Runtime
	jobs *store.JobStore
}

func New(cfg *config.Runtime, jobs *store.JobStore) *Handler {
	return &Handler{cfg: cfg, jobs: jobs}
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	videoRoot, _, _ := h.cfg.Get()
	var videos []models.VideoSummary
	err := filepath.Walk(videoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".mp4") {
			return nil
		}
		rel, err := filepath.Rel(videoRoot, path)
		if err != nil {
			return nil
		}
		meta, err := ffmpeg.Probe(path)
		if err != nil {
			return nil
		}
		videos = append(videos, models.VideoSummary{
			ID:         ffmpeg.EncodeVideoID(rel),
			Name:       info.Name(),
			Path:       rel,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Unix(),
			Duration:   meta.Duration,
			Width:      meta.Width,
			Height:     meta.Height,
			FPS:        meta.FPS,
		})
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if videos == nil {
		videos = []models.VideoSummary{}
	}
	writeJSON(w, videos)
}

func (h *Handler) GetVideo(w http.ResponseWriter, r *http.Request) {
	videoRoot, _, _ := h.cfg.Get()
	id := chi.URLParam(r, "id")
	rel, err := ffmpeg.DecodeVideoID(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	abs, err := ffmpeg.ResolveVideoPath(videoRoot, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	meta, err := ffmpeg.Probe(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, models.VideoSummary{
		ID:       id,
		Name:     filepath.Base(abs),
		Path:     rel,
		Size:     meta.Size,
		Duration: meta.Duration,
		Width:    meta.Width,
		Height:   meta.Height,
		FPS:      meta.FPS,
	})
}

func (h *Handler) StreamVideo(w http.ResponseWriter, r *http.Request) {
	videoRoot, _, _ := h.cfg.Get()
	id := chi.URLParam(r, "id")
	rel, err := ffmpeg.DecodeVideoID(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	abs, err := ffmpeg.ResolveVideoPath(videoRoot, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, abs)
}

func (h *Handler) GetFrame(w http.ResponseWriter, r *http.Request) {
	videoRoot, _, cacheDir := h.cfg.Get()
	id := chi.URLParam(r, "id")
	tStr := r.URL.Query().Get("t")
	if tStr == "" {
		http.Error(w, "missing t parameter", http.StatusBadRequest)
		return
	}
	timestamp, err := strconv.ParseFloat(tStr, 64)
	if err != nil {
		http.Error(w, "invalid t", http.StatusBadRequest)
		return
	}

	rel, err := ffmpeg.DecodeVideoID(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	abs, err := ffmpeg.ResolveVideoPath(videoRoot, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	framePath := filepath.Join(cacheDir, id+"_"+strings.ReplaceAll(tStr, ".", "_")+".jpg")
	if _, err := os.Stat(framePath); os.IsNotExist(err) {
		if err := ffmpeg.ExtractFrame(abs, framePath, timestamp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, framePath)
}

func (h *Handler) Process(w http.ResponseWriter, r *http.Request) {
	var req models.ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	job := &models.Job{
		ID:       jobID,
		Status:   models.JobPending,
		Progress: 0,
		Message:  "Queued",
	}
	h.jobs.Set(job)

	go func() {
		videoRoot, outputDir, _ := h.cfg.Get()
		running := *job
		running.Status = models.JobRunning
		running.Message = "Processing"
		h.jobs.Set(&running)

		result, err := ffmpeg.ProcessProject(videoRoot, outputDir, req, func(p float64, msg string) {
			j, ok := h.jobs.Get(jobID)
			if !ok {
				return
			}
			j.Progress = p
			j.Message = msg
			h.jobs.Set(j)
		})

		j, ok := h.jobs.Get(jobID)
		if !ok {
			return
		}
		if err != nil {
			j.Status = models.JobFailed
			j.Error = err.Error()
			j.Message = "Failed"
		} else {
			j.Status = models.JobCompleted
			j.Progress = 1
			j.Output = result.Output
			j.Manifest = result.Manifest
			j.CacheFolder = result.CacheFolder
			j.Message = "Completed"
		}
		h.jobs.Set(j)
	}()

	writeJSON(w, map[string]string{"jobId": jobID})
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, ok := h.jobs.Get(id)
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, job)
}

func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	videoRoot, outputDir, _ := h.cfg.Get()
	writeJSON(w, map[string]string{
		"videoRoot": videoRoot,
		"outputDir": outputDir,
	})
}

func (h *Handler) SetVideoRoot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		VideoRoot string `json:"videoRoot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.cfg.SetVideoRoot(body.VideoRoot); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	videoRoot, outputDir, _ := h.cfg.Get()
	writeJSON(w, map[string]string{
		"videoRoot": videoRoot,
		"outputDir": outputDir,
	})
}

func (h *Handler) BrowseFolder(w http.ResponseWriter, r *http.Request) {
	path, err := config.BrowseFolder()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.cfg.SetVideoRoot(path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	videoRoot, outputDir, _ := h.cfg.Get()
	writeJSON(w, map[string]string{
		"videoRoot": videoRoot,
		"outputDir": outputDir,
	})
}

func (h *Handler) SaveTimeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		CacheFolder string `json:"cacheFolder"`
		Segments    []struct {
			Start    float64 `json:"start"`
			End      float64 `json:"end"`
			Filename string  `json:"filename"`
			Repeat   int     `json:"repeat"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Segments) == 0 {
		http.Error(w, "no segments to save", http.StatusBadRequest)
		return
	}

	videoRoot, _, _ := h.cfg.Get()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "final-output"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".txt") {
		name += ".txt"
	}
	path := filepath.Join(videoRoot, filepath.Base(name))

	entries := make([]ffmpeg.TimelineFileEntry, len(req.Segments))
	for i, seg := range req.Segments {
		if seg.End <= seg.Start {
			http.Error(w, fmt.Sprintf("invalid segment on line %d: end must be after start", i+1), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(seg.Filename) == "" {
			http.Error(w, fmt.Sprintf("invalid segment on line %d: missing filename", i+1), http.StatusBadRequest)
			return
		}
		entries[i] = ffmpeg.TimelineFileEntry{
			Start:    seg.Start,
			End:      seg.End,
			Filename: strings.TrimSpace(seg.Filename),
			Repeat:   seg.Repeat,
		}
	}

	if err := ffmpeg.WriteTimelineFile(path, req.CacheFolder, entries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"path": path})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
