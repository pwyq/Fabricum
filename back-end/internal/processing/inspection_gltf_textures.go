package processing

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func embeddedGLTFTextures(document glTFDocument, binaryData []byte) ([]TextureFacts, error) {
	textures := make([]TextureFacts, 0, len(document.Images))
	for index, image := range document.Images {
		data, available, err := gltfImageData(document, image, binaryData)
		if err != nil {
			return nil, fmt.Errorf("glTF image %d: %w", index, err)
		}
		if !available {
			continue
		}
		facts, err := imageFacts(data)
		if err != nil {
			if strings.EqualFold(image.MimeType, "image/png") || isPNG(data) {
				return nil, fmt.Errorf("decode embedded image: %w", err)
			}
			continue
		}
		textures = append(textures, TextureFacts{Width: facts.Width, Height: facts.Height})
	}
	return textures, nil
}

func gltfImageData(document glTFDocument, image gltfImage, binaryData []byte) ([]byte, bool, error) {
	if image.BufferView != nil {
		index := *image.BufferView
		if index < 0 || index >= len(document.BufferView) {
			return nil, false, fmt.Errorf("bufferView index %d is out of range", index)
		}
		view := document.BufferView[index]
		if view.Buffer != 0 || view.ByteOffset < 0 || view.ByteLength < 0 {
			return nil, false, errors.New("unsupported or invalid embedded image bufferView")
		}
		if binaryData == nil {
			return nil, false, errors.New("embedded image has no GLB binary chunk")
		}
		if view.ByteOffset > len(binaryData) || view.ByteLength > len(binaryData)-view.ByteOffset {
			return nil, false, errors.New("embedded image bufferView exceeds GLB binary chunk")
		}
		return binaryData[view.ByteOffset : view.ByteOffset+view.ByteLength], true, nil
	}
	if image.URI == "" || !strings.HasPrefix(strings.ToLower(image.URI), "data:") {
		return nil, false, nil
	}
	data, err := decodeDataURI(image.URI)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func decodeDataURI(value string) ([]byte, error) {
	comma := strings.IndexByte(value, ',')
	if comma < 0 {
		return nil, errors.New("invalid data URI")
	}
	metadata, payload := value[:comma], value[comma+1:]
	if strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("decode base64 data URI: %w", err)
		}
		return data, nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return nil, fmt.Errorf("decode data URI: %w", err)
	}
	return []byte(decoded), nil
}
