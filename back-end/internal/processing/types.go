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

// SourceConstraints limits the inputs accepted by a deterministic transform.
// A nil HasAlpha means that alpha presence is not constrained.
type SourceConstraints struct {
	Format      string `json:"format,omitempty"`
	SingleFrame bool   `json:"singleFrame,omitempty"`
	HasAlpha    *bool  `json:"hasAlpha,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// ResizeSpec describes an exact-size resize. Fill stretches the current image
// to the requested dimensions; contain preserves its aspect ratio and places
// the result on a transparent canvas of the requested size.
type ResizeSpec struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Fit    string `json:"fit,omitempty"`
	Filter string `json:"filter,omitempty"`
}

// PaddingSpec adds transparent pixels around an image.
type PaddingSpec struct {
	Top    int `json:"top,omitempty"`
	Right  int `json:"right,omitempty"`
	Bottom int `json:"bottom,omitempty"`
	Left   int `json:"left,omitempty"`
}

// ChannelInput supplies one channel for a channel pack. Set Constant for a
// literal byte value, or Source and Channel to read a source image channel.
// An empty Source reads from the transform request's primary source.
type ChannelInput struct {
	Source   string `json:"source,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Constant *uint8 `json:"constant,omitempty"`
}

// ChannelPack builds an opaque RGB image from three independently supplied
// channels.
type ChannelPack struct {
	Red   ChannelInput `json:"red"`
	Green ChannelInput `json:"green"`
	Blue  ChannelInput `json:"blue"`
}

// ImageTransform is the project-neutral operation set for one output. Spatial
// operations run in the order crop, resize, and padding. Channel operations
// run after those spatial operations; Pack takes precedence over Channel.
type ImageTransform struct {
	Crop        *CropRect    `json:"crop,omitempty"`
	Resize      *ResizeSpec  `json:"resize,omitempty"`
	Padding     *PaddingSpec `json:"padding,omitempty"`
	RemoveAlpha bool         `json:"removeAlpha,omitempty"`
	Grayscale   bool         `json:"grayscale,omitempty"`
	Channel     string       `json:"channel,omitempty"`
	Pack        *ChannelPack `json:"pack,omitempty"`
}

// TransformOutputSpec identifies one deterministic output.
type TransformOutputSpec struct {
	Role      string         `json:"role,omitempty"`
	Path      string         `json:"path"`
	Transform ImageTransform `json:"transform"`
}

// TransformRequest describes a noninteractive image transformation. The
// output format and encoder options apply to every output in the request.
type TransformRequest struct {
	Source            string                `json:"source"`
	Constraints       SourceConstraints     `json:"constraints,omitempty"`
	SourceConstraints *SourceConstraints    `json:"sourceConstraints,omitempty"`
	Outputs           []TransformOutputSpec `json:"outputs"`
	Format            string                `json:"format"`
	Quality           int                   `json:"quality,omitempty"`
	Lossless          bool                  `json:"lossless,omitempty"`
	EncoderDirectory  string                `json:"encoderDirectory,omitempty"`
}

// TextureEncodingOptions describes one loose KTX2 encoding. MipLevels is
// optional; when omitted, the encoder writes the complete generated mip chain.
type TextureEncodingOptions struct {
	Encoding         string `json:"encoding,omitempty"`
	Mode             string `json:"mode,omitempty"`
	MipLevels        int    `json:"mipLevels,omitempty"`
	TransferFunction string `json:"transferFunction,omitempty"`
	ColorPrimaries   string `json:"colorPrimaries,omitempty"`
	Quality          int    `json:"quality,omitempty"`
	ZstdLevel        int    `json:"zstdLevel,omitempty"`
}

// TextureOutputSpec describes one loose KTX2 output. Transform uses the same
// project-neutral crop, resize, padding, and channel-packing operations as a
// regular transform request.
type TextureOutputSpec struct {
	Role              string                 `json:"role,omitempty"`
	Path              string                 `json:"path"`
	Source            string                 `json:"source"`
	Constraints       SourceConstraints      `json:"constraints,omitempty"`
	SourceConstraints *SourceConstraints     `json:"sourceConstraints,omitempty"`
	Transform         ImageTransform         `json:"transform,omitempty"`
	Format            string                 `json:"format,omitempty"`
	Encoding          TextureEncodingOptions `json:"encoding"`
}

// TextureSetRequest prepares one or more independent loose KTX2 outputs.
// RequiredRoles makes a material set fail before any output is replaced when
// one of its caller-defined roles is absent.
type TextureSetRequest struct {
	Outputs          []TextureOutputSpec `json:"outputs"`
	RequiredRoles    []string            `json:"requiredRoles,omitempty"`
	MaxWorkers       int                 `json:"maxWorkers,omitempty"`
	EncoderDirectory string              `json:"encoderDirectory,omitempty"`
}

const (
	TextureEncodingETC1S     = "etc1s"
	TextureEncodingUASTCZstd = "uastc-zstd"
	TextureTransferLinear    = "linear"
	TextureTransferSRGB      = "srgb"
	TexturePrimariesBT709    = "bt709"
)

// These aliases keep the KTX2/material terminology available to integrations
// without adding a second request format.
type KTX2EncodingOptions = TextureEncodingOptions
type KTX2OutputSpec = TextureOutputSpec
type KTX2SetRequest = TextureSetRequest
type MaterialSetRequest = TextureSetRequest

type OutputSpec struct {
	Role   string
	Path   string
	Width  int
	Height int
}

type OutputMeasurement struct {
	Role                  string            `json:"role"`
	Path                  string            `json:"path"`
	Width                 int               `json:"width"`
	Height                int               `json:"height"`
	Format                string            `json:"format"`
	Encoder               string            `json:"encoder"`
	EncoderVersion        string            `json:"encoderVersion,omitempty"`
	Processor             string            `json:"processor,omitempty"`
	NativeEncoderVersions map[string]string `json:"nativeEncoderVersions,omitempty"`
	HasAlpha              bool              `json:"hasAlpha"`
	Filter                string            `json:"filter,omitempty"`
	Quality               int               `json:"quality,omitempty"`
	Lossless              bool              `json:"lossless,omitempty"`
	Encoding              string            `json:"encoding,omitempty"`
	MipLevels             int               `json:"mipLevels,omitempty"`
	TransferFunction      string            `json:"transferFunction,omitempty"`
	ColorPrimaries        string            `json:"colorPrimaries,omitempty"`
	Bytes                 int               `json:"bytes"`
	SHA256                string            `json:"sha256"`
	Crop                  CropRect          `json:"crop"`
}

type ProcessedOutput struct {
	Measurement OutputMeasurement
	Path        string
	Data        []byte
}
