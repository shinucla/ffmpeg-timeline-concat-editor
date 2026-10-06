package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/shin/web-load-time-cut-concat/backend/config"
	"github.com/shin/web-load-time-cut-concat/backend/handlers"
	"github.com/shin/web-load-time-cut-concat/backend/store"
)

func skipRequestLog(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/jobs/") {
		return true
	}
	if strings.HasPrefix(path, "/api/videos/") && strings.HasSuffix(path, "/stream") {
		return true
	}
	return false
}

func selectiveRequestLogger() func(http.Handler) http.Handler {
	stdLogger := middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger: log.Default(),
	})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipRequestLog(r) {
				next.ServeHTTP(w, r)
				return
			}
			stdLogger(next).ServeHTTP(w, r)
		})
	}
}

func main() {
	cfg := config.LoadRuntime()
	jobs := store.NewJobStore()
	h := handlers.New(cfg, jobs)

	r := chi.NewRouter()
	r.Use(selectiveRequestLogger())
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/api/config", h.Config)
	r.Post("/api/config/video-root", h.SetVideoRoot)
	r.Post("/api/config/browse-folder", h.BrowseFolder)
	r.Get("/api/videos", h.ListVideos)
	r.Get("/api/videos/{id}", h.GetVideo)
	r.Get("/api/videos/{id}/stream", h.StreamVideo)
	r.Get("/api/videos/{id}/frame", h.GetFrame)
	r.Post("/api/process", h.Process)
	r.Post("/api/ytdlp/download", h.DownloadYtdlp)
	r.Post("/api/timeline/save", h.SaveTimeline)
	r.Get("/api/jobs/{id}", h.GetJob)

	videoRoot, outputDir, _ := cfg.Get()
	log.Printf("video root: %s", videoRoot)
	log.Printf("output dir: %s", outputDir)
	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
