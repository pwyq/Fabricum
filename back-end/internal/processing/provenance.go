package processing

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
	default:
		return ""
	}
}
