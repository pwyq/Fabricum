package processing

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

func repairModelWinding(document *modelDocument) error {
	meshes, err := modelArray(document.Object, "meshes")
	if err != nil {
		return err
	}
	reversed := make(map[int]struct{})
	for meshIndex, mesh := range meshes {
		primitives, err := modelArray(mesh, "primitives")
		if err != nil {
			return err
		}
		for primitiveIndex, primitive := range primitives {
			mode := 4
			if raw, ok := primitive["mode"]; ok {
				value, err := modelIntValue(raw)
				if err != nil {
					return err
				}
				mode = value
			}
			if mode != 4 {
				return fmt.Errorf("mirrored winding repair only supports triangle primitives (mesh %d primitive %d uses mode %d)", meshIndex, primitiveIndex, mode)
			}
			if raw, ok := primitive["indices"]; ok {
				index, err := modelIndexValue(raw)
				if err != nil {
					return err
				}
				if _, done := reversed[index]; !done {
					if err := reverseModelIndexAccessor(document, index); err != nil {
						return fmt.Errorf("mesh %d primitive %d: %w", meshIndex, primitiveIndex, err)
					}
					reversed[index] = struct{}{}
				}
				continue
			}
			attributes, _ := modelObject(primitive, "attributes")
			positionRaw, ok := attributes["POSITION"]
			if !ok {
				continue
			}
			positionIndex, err := modelIndexValue(positionRaw)
			if err != nil {
				return err
			}
			accessors, _ := modelArray(document.Object, "accessors")
			count, _, err := modelInt(accessors[positionIndex], "count")
			if err != nil || count%3 != 0 {
				return fmt.Errorf("mesh %d primitive %d has a non-triangular non-indexed POSITION accessor", meshIndex, primitiveIndex)
			}
			index, err := appendModelIndices(document, count)
			if err != nil {
				return err
			}
			if err := reverseModelIndexAccessor(document, index); err != nil {
				return fmt.Errorf("mesh %d primitive %d: %w", meshIndex, primitiveIndex, err)
			}
			if err := setModelValue(primitive, "indices", index); err != nil {
				return err
			}
		}
		if err := setModelArray(mesh, "primitives", primitives); err != nil {
			return err
		}
	}
	return setModelArray(document.Object, "meshes", meshes)
}

func modelIntValue(raw json.RawMessage) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, errors.New("value must be an integer")
	}
	return value, nil
}

func modelIndexValue(raw json.RawMessage) (int, error) {
	return modelIntValue(raw)
}

func reverseModelIndexAccessor(document *modelDocument, accessorIndex int) error {
	accessors, err := modelArray(document.Object, "accessors")
	if err != nil {
		return err
	}
	views, err := modelArray(document.Object, "bufferViews")
	if err != nil {
		return err
	}
	if accessorIndex < 0 || accessorIndex >= len(accessors) {
		return errors.New("index accessor is out of range")
	}
	accessor := accessors[accessorIndex]
	typeName, err := modelString(accessor, "type")
	if err != nil || typeName != "SCALAR" {
		return errors.New("index accessor must be SCALAR")
	}
	count, present, err := modelInt(accessor, "count")
	if err != nil || !present || count < 0 || count%3 != 0 {
		return errors.New("index accessor count must be a non-negative multiple of three")
	}
	componentType, present, err := modelInt(accessor, "componentType")
	if err != nil || !present {
		return errors.New("index accessor componentType is required")
	}
	componentSize := map[int]int{5121: 1, 5123: 2, 5125: 4}[componentType]
	if componentSize == 0 {
		return fmt.Errorf("unsupported index componentType %d", componentType)
	}
	viewIndex, present, err := modelInt(accessor, "bufferView")
	if err != nil || !present || viewIndex < 0 || viewIndex >= len(views) {
		return errors.New("index accessor bufferView is invalid")
	}
	view := views[viewIndex]
	bufferIndex, present, err := modelInt(view, "buffer")
	if err != nil || !present || bufferIndex < 0 || bufferIndex >= len(document.Buffers) {
		return errors.New("index accessor buffer is invalid")
	}
	viewOffset, _, err := modelInt(view, "byteOffset")
	if err != nil || viewOffset < 0 {
		return errors.New("index bufferView byteOffset is invalid")
	}
	accessorOffset, _, err := modelInt(accessor, "byteOffset")
	if err != nil || accessorOffset < 0 {
		return errors.New("index accessor byteOffset is invalid")
	}
	stride, _, err := modelInt(view, "byteStride")
	if err != nil {
		return errors.New("index bufferView byteStride is invalid")
	}
	if stride == 0 {
		stride = componentSize
	}
	if stride < componentSize {
		return errors.New("index bufferView byteStride is too small")
	}
	start := viewOffset + accessorOffset
	if start < 0 || count > 0 && (count-1)*stride+componentSize > len(document.Buffers[bufferIndex])-start {
		return errors.New("index accessor exceeds its buffer")
	}
	for index := 0; index < count; index += 3 {
		first := start + (index+1)*stride
		second := start + (index+2)*stride
		left := readModelIndex(document.Buffers[bufferIndex][first:first+componentSize], componentType)
		right := readModelIndex(document.Buffers[bufferIndex][second:second+componentSize], componentType)
		writeModelIndex(document.Buffers[bufferIndex][first:first+componentSize], componentType, right)
		writeModelIndex(document.Buffers[bufferIndex][second:second+componentSize], componentType, left)
	}
	return nil
}

func readModelIndex(data []byte, componentType int) uint32 {
	switch componentType {
	case 5121:
		return uint32(data[0])
	case 5123:
		return uint32(binary.LittleEndian.Uint16(data))
	default:
		return binary.LittleEndian.Uint32(data)
	}
}

func writeModelIndex(data []byte, componentType int, value uint32) {
	switch componentType {
	case 5121:
		data[0] = byte(value)
	case 5123:
		binary.LittleEndian.PutUint16(data, uint16(value))
	default:
		binary.LittleEndian.PutUint32(data, value)
	}
}

func appendModelIndices(document *modelDocument, count int) (int, error) {
	if len(document.Buffers) == 0 {
		document.Buffers = append(document.Buffers, nil)
	}
	data := document.Buffers[0]
	data = append(data, make([]byte, (4-len(data)%4)%4)...)
	byteOffset := len(data)
	for index := 0; index < count; index++ {
		var encoded [4]byte
		binary.LittleEndian.PutUint32(encoded[:], uint32(index))
		data = append(data, encoded[:]...)
	}
	document.Buffers[0] = data
	views, err := modelArray(document.Object, "bufferViews")
	if err != nil {
		return 0, err
	}
	viewIndex := len(views)
	views = append(views, map[string]json.RawMessage{})
	if err := setModelValue(views[viewIndex], "buffer", 0); err != nil {
		return 0, err
	}
	if err := setModelValue(views[viewIndex], "byteOffset", byteOffset); err != nil {
		return 0, err
	}
	if err := setModelValue(views[viewIndex], "byteLength", count*4); err != nil {
		return 0, err
	}
	if err := setModelValue(views[viewIndex], "target", 34963); err != nil {
		return 0, err
	}
	if err := setModelArray(document.Object, "bufferViews", views); err != nil {
		return 0, err
	}
	accessors, err := modelArray(document.Object, "accessors")
	if err != nil {
		return 0, err
	}
	accessorIndex := len(accessors)
	accessors = append(accessors, map[string]json.RawMessage{})
	for key, value := range map[string]any{"bufferView": viewIndex, "componentType": 5125, "count": count, "type": "SCALAR"} {
		if err := setModelValue(accessors[accessorIndex], key, value); err != nil {
			return 0, err
		}
	}
	if count > 0 {
		if err := setModelValue(accessors[accessorIndex], "min", []int{0}); err != nil {
			return 0, err
		}
		if err := setModelValue(accessors[accessorIndex], "max", []int{count - 1}); err != nil {
			return 0, err
		}
	}
	if err := setModelArray(document.Object, "accessors", accessors); err != nil {
		return 0, err
	}
	return accessorIndex, nil
}
