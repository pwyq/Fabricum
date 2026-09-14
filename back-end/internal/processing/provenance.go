package processing

const (
	BasisUniversalVersion = "2.0.3"
	LibAVIFVersion        = "1.4.2"
	LibAOMVersion         = "3.14.1"
	LibWebPVersion        = "1.6.0"
	ProcessorName         = "fabricum"
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
	switch format {
	case "webp":
		return map[string]string{"cwebp": LibWebPVersion}
	case "avif":
		return map[string]string{"avifenc": LibAVIFVersion, "libaom": LibAOMVersion}
	case "ktx2":
		return map[string]string{"basisu": BasisUniversalVersion}
	default:
		return nil
	}
}

// EncoderVersionForFormat identifies the primary bundled encoder version for
// one output format. Formats with no separately pinned encoder return empty.
func EncoderVersionForFormat(format string) string {
	switch format {
	case "webp":
		return LibWebPVersion
	case "avif":
		return LibAVIFVersion
	case "ktx2":
		return BasisUniversalVersion
	default:
		return ""
	}
}
