package models

type VideoSummary struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	ModifiedAt int64   `json:"modifiedAt"`
	Duration   float64 `json:"duration"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
}

type Segment struct {
	Start                  float64 `json:"start"`
	End                    float64 `json:"end"`
	Repeat                 int     `json:"repeat,omitempty"`
	RotationSteps          int     `json:"rotationSteps,omitempty"`
	AlternateRepeatReverse bool    `json:"alternateRepeatReverse,omitempty"`
}

type Clip struct {
	ID       string    `json:"id"`
	VideoID  string    `json:"videoId"`
	Order    int       `json:"order"`
	Segments []Segment `json:"segments"`
}

type ProcessRequest struct {
	Name           string `json:"name"`
	Clips          []Clip `json:"clips"`
	CacheFolder    string `json:"cacheFolder,omitempty"`
	ExportCutParts bool   `json:"exportCutParts"`
}

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

type Job struct {
	ID          string    `json:"id"`
	Status      JobStatus `json:"status"`
	Progress    float64   `json:"progress"`
	Message     string    `json:"message"`
	Output      string    `json:"output,omitempty"`
	Manifest    string    `json:"manifest,omitempty"`
	CacheFolder string    `json:"cacheFolder,omitempty"`
	Error       string    `json:"error,omitempty"`
}
