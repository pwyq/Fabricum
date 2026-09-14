package processing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type modelDocument struct {
	Object     map[string]json.RawMessage
	Buffers    [][]byte
	SourcePath string
	SourceDir  string
}

func loadModelDocument(path string) (modelDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return modelDocument{}, fmt.Errorf("read model: %w", err)
	}
	var jsonData, binaryData []byte
	switch strings.ToLower(filepath.Ext(path)) {
	case ".glb":
		jsonData, binaryData, err = parseModelGLB(data)
	case ".gltf":
		jsonData = data
	default:
		err = fmt.Errorf("model path must end in .gltf or .glb")
	}
	if err != nil {
		return modelDocument{}, err
	}
	object, err := decodeModelObject(jsonData)
	if err != nil {
		return modelDocument{}, err
	}
	asset, err := modelObject(object, "asset")
	if err != nil {
		return modelDocument{}, fmt.Errorf("model asset: %w", err)
	}
	if version, err := modelString(asset, "version"); err != nil || version != "2.0" {
		if err != nil {
			return modelDocument{}, fmt.Errorf("model asset version: %w", err)
		}
		return modelDocument{}, errors.New("model asset version must be 2.0")
	}
	document := modelDocument{Object: object, SourcePath: path, SourceDir: filepath.Dir(path)}
	if err := document.readBuffers(binaryData); err != nil {
		return modelDocument{}, err
	}
	return document, nil
}

func parseModelGLB(data []byte) ([]byte, []byte, error) {
	if !isGLB(data) || len(data) < 12 || readUint32LE(data[4:8]) != 2 {
		return nil, nil, errors.New("invalid GLB header")
	}
	if uint64(readUint32LE(data[8:12])) != uint64(len(data)) {
		return nil, nil, errors.New("GLB length does not match file")
	}
	var jsonData, binaryData []byte
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, nil, errors.New("truncated GLB chunk")
		}
		length := int(readUint32LE(data[offset : offset+4]))
		if length%4 != 0 || length > len(data)-offset-8 {
			return nil, nil, errors.New("invalid GLB chunk length")
		}
		chunk := data[offset+8 : offset+8+length]
		switch readUint32LE(data[offset+4 : offset+8]) {
		case 0x4e4f534a:
			if jsonData != nil || offset != 12 {
				return nil, nil, errors.New("GLB must contain one JSON chunk first")
			}
			jsonData = bytes.TrimRight(chunk, "\x00 \t\r\n")
		case 0x004e4942:
			if binaryData != nil {
				return nil, nil, errors.New("GLB contains multiple binary chunks")
			}
			binaryData = chunk
		}
		offset += 8 + length
	}
	if jsonData == nil {
		return nil, nil, errors.New("GLB is missing its JSON chunk")
	}
	return jsonData, binaryData, nil
}

func readUint32LE(data []byte) uint32 {
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

func decodeModelObject(data []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(data), &object); err != nil {
		return nil, fmt.Errorf("decode glTF JSON: %w", err)
	}
	if object == nil {
		return nil, errors.New("glTF JSON must contain an object")
	}
	return object, nil
}

func (document *modelDocument) readBuffers(binaryData []byte) error {
	entries, err := modelArray(document.Object, "buffers")
	if err != nil {
		return fmt.Errorf("model buffers: %w", err)
	}
	if len(entries) == 0 && len(binaryData) != 0 {
		return errors.New("GLB binary chunk has no buffer declaration")
	}
	document.Buffers = make([][]byte, len(entries))
	for index, entry := range entries {
		uri, _ := modelString(entry, "uri")
		var data []byte
		switch {
		case strings.HasPrefix(strings.ToLower(uri), "data:"):
			data, err = decodeDataURI(uri)
		case uri != "":
			data, err = os.ReadFile(modelRelativePath(document.SourceDir, uri))
		case index == 0 && binaryData != nil:
			data = binaryData
		default:
			err = errors.New("buffer has no URI or GLB binary data")
		}
		if err != nil {
			return fmt.Errorf("read buffer %d: %w", index, err)
		}
		byteLength, present, lengthErr := modelInt(entry, "byteLength")
		if lengthErr != nil || !present || byteLength < 0 || byteLength > len(data) {
			return fmt.Errorf("buffer %d has an invalid byteLength", index)
		}
		document.Buffers[index] = data
	}
	return nil
}

func modelRelativePath(base, uri string) string {
	decoded, err := url.PathUnescape(uri)
	if err != nil {
		decoded = uri
	}
	return filepath.Join(base, filepath.FromSlash(decoded))
}
