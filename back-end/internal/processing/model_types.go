package processing

const (
	ModelCompressionNone    = "none"
	ModelCompressionMeshopt = "meshopt"
	ModelTextureNone        = "none"
	ModelTextureKTX2        = "ktx2"
	ModelTextureWebP        = "webp"
	ModelTextureETC1S       = "etc1s"
	ModelTextureUASTC       = "uastc"
	GltfpackVersion         = "1.2"
)

// ModelOptimizationRequest describes one project-neutral static model build.
// Output and material names are supplied by the caller; Fabricum does not
// attach project-specific naming or delivery policy.
type ModelOptimizationRequest struct {
	Source               string            `json:"source"`
	Output               string            `json:"output"`
	Role                 string            `json:"role,omitempty"`
	Compression          string            `json:"compression,omitempty"`
	MeshCompression      string            `json:"meshCompression,omitempty"`
	TextureCompression   string            `json:"textureCompression,omitempty"`
	TextureEncoding      string            `json:"textureEncoding,omitempty"`
	TextureQuality       int               `json:"textureQuality,omitempty"`
	BakeRootTransform    bool              `json:"bakeRootTransform,omitempty"`
	CenterXZAtGround     bool              `json:"centerXZAtGround,omitempty"`
	RemoveAttributes     []string          `json:"removeAttributes,omitempty"`
	DeduplicateMaterials bool              `json:"deduplicateMaterials,omitempty"`
	CompactBuffers       bool              `json:"compactBuffers,omitempty"`
	MaterialNames        map[string]string `json:"materialNames,omitempty"`
	GltfpackDirectory    string            `json:"gltfpackDirectory,omitempty"`
	EncoderDirectory     string            `json:"encoderDirectory,omitempty"`
}

// StaticPropRequest is a descriptive alias for callers that process only
// static props.
type StaticPropRequest = ModelOptimizationRequest

// ModelOutputMeasurement describes the main model output or a referenced
// sidecar emitted for a loose glTF output.
type ModelOutputMeasurement struct {
	Role               string            `json:"role,omitempty"`
	Path               string            `json:"path"`
	Format             string            `json:"format"`
	Tool               string            `json:"tool"`
	ToolVersion        string            `json:"toolVersion"`
	Bytes              int               `json:"bytes"`
	SHA256             string            `json:"sha256"`
	Bounds             *Bounds           `json:"bounds,omitempty"`
	TriangleCount      int               `json:"triangleCount"`
	PrimitiveCount     int               `json:"primitiveCount"`
	MaterialCount      int               `json:"materialCount"`
	TextureCount       int               `json:"textureCount"`
	AnimationCount     int               `json:"animationCount"`
	UsedExtensions     []string          `json:"usedExtensions,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
	NativeToolVersions map[string]string `json:"nativeToolVersions,omitempty"`
}

// ModelOptimizationResult contains prepared model files and their receipt
// measurements. It is useful to integrations that need to inspect output
// bytes before replacing existing deliveries.
type ModelOptimizationResult struct {
	Outputs  []ProcessedModelOutput
	Warnings []string
}

type ProcessedModelOutput struct {
	Measurement ModelOutputMeasurement
	Path        string
	Data        []byte
}
