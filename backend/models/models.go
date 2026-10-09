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

// Watermark is a burned-in text overlay on a segment.
// Position/size are fractions (0..1) of the output frame; Start/Duration are
// seconds relative to the start of the owning segment.
type Watermark struct {
	Enabled  bool    `json:"enabled"`
	Text     string  `json:"text"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}

type Segment struct {
	Start                  float64    `json:"start"`
	End                    float64    `json:"end"`
	Repeat                 int        `json:"repeat,omitempty"`
	RotationSteps          int        `json:"rotationSteps,omitempty"`
	AlternateRepeatReverse bool       `json:"alternateRepeatReverse,omitempty"`
	Watermark              *Watermark `json:"watermark,omitempty"`
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

type YtdlpSegment struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type YtdlpDownloadRequest struct {
	URL      string         `json:"url"`
	Segments []YtdlpSegment `json:"segments"`
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
