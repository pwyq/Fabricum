package processing

type CropRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ExportRequest struct {
	Role     string   `json:"role,omitempty"`
	Square   CropRect `json:"square"`
	Wide     CropRect `json:"wide"`
	Format   string   `json:"format"`
	Quality  int      `json:"quality"`
	Lossless bool     `json:"lossless"`
}

type OutputSpec struct {
	Role   string
	Path   string
	Width  int
	Height int
}

type OutputMeasurement struct {
	Role     string   `json:"role"`
	Path     string   `json:"path"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Format   string   `json:"format"`
	Quality  int      `json:"quality,omitempty"`
	Lossless bool     `json:"lossless,omitempty"`
	Bytes    int      `json:"bytes"`
	SHA256   string   `json:"sha256"`
	Crop     CropRect `json:"crop"`
}

type ProcessedOutput struct {
	Measurement OutputMeasurement
	Path        string
	Data        []byte
}
