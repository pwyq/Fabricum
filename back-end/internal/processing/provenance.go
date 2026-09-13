package processing

const (
	BasisUniversalVersion = "2.0.3"
	LibWebPVersion        = "1.6.0"
)

// EncoderForFormat identifies the encoder that produced a successful output.
// Native names include the codec library selected by Fabricum's contract.
func EncoderForFormat(format string) string {
	switch format {
	case "png":
		return "go/image/png"
	case "webp":
		return "cwebp/libwebp"
	case "avif":
		return "avifenc/libavif+libaom"
	case "ktx2":
		return "basisu/Basis Universal"
	default:
		return ""
	}
}

// EncoderVersionsForFormat returns the pinned native encoder versions used by
// a format. A fresh map is returned so receipt callers cannot mutate shared
// provenance.
func EncoderVersionsForFormat(format string) map[string]string {
	if format != "ktx2" {
		return nil
	}
	return map[string]string{"basisu": BasisUniversalVersion}
}
