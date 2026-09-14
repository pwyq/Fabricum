package processing

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

type modelPositionAccessor struct {
	index       int
	object      map[string]json.RawMessage
	buffer      []byte
	bufferIndex int
	start       int
	stride      int
	count       int
}

func positionAccessors(object map[string]json.RawMessage, buffers [][]byte) ([]modelPositionAccessor, error) {
	accessors, err := modelArray(object, "accessors")
	if err != nil {
		return nil, err
	}
	views, err := modelArray(object, "bufferViews")
	if err != nil {
		return nil, err
	}
	meshes, err := modelArray(object, "meshes")
	if err != nil {
		return nil, err
	}
	used := make(map[int]struct{})
	for _, mesh := range meshes {
		primitives, _ := modelArray(mesh, "primitives")
		for _, primitive := range primitives {
			attributes, _ := modelObject(primitive, "attributes")
			if raw, ok := attributes["POSITION"]; ok {
				index, err := modelIndex(raw, len(accessors))
				if err != nil {
					return nil, err
				}
				used[index] = struct{}{}
			}
		}
	}
	indices := make([]int, 0, len(used))
	for index := range used {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	result := make([]modelPositionAccessor, 0, len(indices))
	for _, index := range indices {
		accessor := accessors[index]
		typeName, err := modelString(accessor, "type")
		if err != nil || typeName != "VEC3" {
			return nil, fmt.Errorf("POSITION accessor %d must be VEC3", index)
		}
		componentType, present, err := modelInt(accessor, "componentType")
		if err != nil || !present || componentType != 5126 {
			return nil, fmt.Errorf("POSITION accessor %d must use FLOAT components", index)
		}
		count, present, err := modelInt(accessor, "count")
		if err != nil || !present || count < 0 {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid count", index)
		}
		viewIndex, present, err := modelInt(accessor, "bufferView")
		if err != nil || !present || viewIndex < 0 || viewIndex >= len(views) {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid bufferView", index)
		}
		view := views[viewIndex]
		bufferIndex, present, err := modelInt(view, "buffer")
		if err != nil || !present || bufferIndex < 0 || bufferIndex >= len(buffers) {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid buffer", index)
		}
		viewOffset, _, err := modelInt(view, "byteOffset")
		if err != nil || viewOffset < 0 {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid byteOffset", index)
		}
		accessorOffset, _, err := modelInt(accessor, "byteOffset")
		if err != nil || accessorOffset < 0 {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid accessor offset", index)
		}
		stride, _, err := modelInt(view, "byteStride")
		if err != nil {
			return nil, fmt.Errorf("POSITION accessor %d has an invalid byteStride", index)
		}
		if stride == 0 {
			stride = 12
		}
		if stride < 12 {
			return nil, fmt.Errorf("POSITION accessor %d byteStride is too small", index)
		}
		start := viewOffset + accessorOffset
		viewLength, _, err := modelInt(view, "byteLength")
		if err != nil || viewLength < 0 || start > viewOffset+viewLength || count > 0 && (count-1)*stride+12 > viewLength-accessorOffset {
			return nil, fmt.Errorf("POSITION accessor %d exceeds its bufferView", index)
		}
		if start > len(buffers[bufferIndex]) || count > 0 && (count-1)*stride+12 > len(buffers[bufferIndex])-start {
			return nil, fmt.Errorf("POSITION accessor %d exceeds its buffer", index)
		}
		result = append(result, modelPositionAccessor{index: index, object: accessor, buffer: buffers[bufferIndex], bufferIndex: bufferIndex, start: start, stride: stride, count: count})
	}
	return result, nil
}

func transformModelPositions(document *modelDocument, matrix modelMatrix, center bool) error {
	positions, err := positionAccessors(document.Object, document.Buffers)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}
	var bounds Bounds
	initialized := false
	positionBounds := make([]Bounds, len(positions))
	positionInitialized := make([]bool, len(positions))
	for positionIndex, position := range positions {
		for index := 0; index < position.count; index++ {
			x, y, z := modelPosition(position, index)
			x, y, z = modelTransformPoint(matrix, x, y, z)
			if !positionInitialized[positionIndex] {
				positionBounds[positionIndex].Min, positionBounds[positionIndex].Max = [3]float64{x, y, z}, [3]float64{x, y, z}
				positionInitialized[positionIndex] = true
			} else {
				positionBounds[positionIndex].Min[0], positionBounds[positionIndex].Min[1], positionBounds[positionIndex].Min[2] = minFloat(positionBounds[positionIndex].Min[0], x), minFloat(positionBounds[positionIndex].Min[1], y), minFloat(positionBounds[positionIndex].Min[2], z)
				positionBounds[positionIndex].Max[0], positionBounds[positionIndex].Max[1], positionBounds[positionIndex].Max[2] = maxFloat(positionBounds[positionIndex].Max[0], x), maxFloat(positionBounds[positionIndex].Max[1], y), maxFloat(positionBounds[positionIndex].Max[2], z)
			}
			if !initialized {
				bounds.Min, bounds.Max = [3]float64{x, y, z}, [3]float64{x, y, z}
				initialized = true
				continue
			}
			bounds.Min[0], bounds.Min[1], bounds.Min[2] = minFloat(bounds.Min[0], x), minFloat(bounds.Min[1], y), minFloat(bounds.Min[2], z)
			bounds.Max[0], bounds.Max[1], bounds.Max[2] = maxFloat(bounds.Max[0], x), maxFloat(bounds.Max[1], y), maxFloat(bounds.Max[2], z)
		}
	}
	if !initialized {
		return errors.New("model contains no POSITION data")
	}
	translation := [3]float64{}
	if center {
		translation = [3]float64{(bounds.Min[0] + bounds.Max[0]) / 2, bounds.Min[1], (bounds.Min[2] + bounds.Max[2]) / 2}
	}
	for index, position := range positions {
		for vertex := 0; vertex < position.count; vertex++ {
			x, y, z := modelPosition(position, vertex)
			x, y, z = modelTransformPoint(matrix, x, y, z)
			modelWritePosition(position, vertex, x-translation[0], y-translation[1], z-translation[2])
		}
		minimum := [3]float64{positionBounds[index].Min[0] - translation[0], positionBounds[index].Min[1] - translation[1], positionBounds[index].Min[2] - translation[2]}
		maximum := [3]float64{positionBounds[index].Max[0] - translation[0], positionBounds[index].Max[1] - translation[1], positionBounds[index].Max[2] - translation[2]}
		if err := setModelValue(position.object, "min", minimum[:]); err != nil {
			return err
		}
		if err := setModelValue(position.object, "max", maximum[:]); err != nil {
			return err
		}
	}
	accessors, err := modelArray(document.Object, "accessors")
	if err != nil {
		return err
	}
	for _, position := range positions {
		accessors[position.index] = position.object
	}
	return setModelArray(document.Object, "accessors", accessors)
}

func modelPosition(position modelPositionAccessor, index int) (float64, float64, float64) {
	offset := position.start + index*position.stride
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(position.buffer[offset:]))),
		float64(math.Float32frombits(binary.LittleEndian.Uint32(position.buffer[offset+4:]))),
		float64(math.Float32frombits(binary.LittleEndian.Uint32(position.buffer[offset+8:])))
}

func modelWritePosition(position modelPositionAccessor, index int, x, y, z float64) {
	offset := position.start + index*position.stride
	binary.LittleEndian.PutUint32(position.buffer[offset:], math.Float32bits(float32(x)))
	binary.LittleEndian.PutUint32(position.buffer[offset+4:], math.Float32bits(float32(y)))
	binary.LittleEndian.PutUint32(position.buffer[offset+8:], math.Float32bits(float32(z)))
}
