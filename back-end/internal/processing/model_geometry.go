package processing

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

type modelMatrix [16]float64

type modelPositionAccessor struct {
	index       int
	object      map[string]json.RawMessage
	buffer      []byte
	bufferIndex int
	start       int
	stride      int
	count       int
}

func prepareModelGeometry(document *modelDocument, request ModelOptimizationRequest) ([]string, error) {
	if err := validateModelReferences(document.Object); err != nil {
		return nil, err
	}
	warnings := make([]string, 0)
	if err := applyMaterialNames(document.Object, request.MaterialNames); err != nil {
		return nil, err
	}
	if request.DeduplicateMaterials {
		if err := deduplicateModelMaterials(document.Object); err != nil {
			return nil, err
		}
	}
	if removed := removeModelAttributes(document.Object, request.RemoveAttributes); removed > 0 {
		warnings = append(warnings, fmt.Sprintf("discarded %d caller-selected vertex attributes", removed))
	}
	transform := identityModelMatrix()
	mirrored := false
	if request.BakeRootTransform {
		rootTransform, roots, err := modelRootTransform(document.Object)
		if err != nil {
			return nil, err
		}
		transform = rootTransform
		mirrored = modelMatrixDeterminant(transform) < 0
		if roots {
			clearModelRootTransforms(document.Object)
		}
		if len(modelAnimations(document.Object)) > 0 && !modelMatrixIsIdentity(transform) {
			return nil, errors.New("cannot bake a root transform on an animated model")
		}
	}
	if mirrored {
		if err := repairModelWinding(document); err != nil {
			return nil, err
		}
	}
	if request.CenterXZAtGround || !modelMatrixIsIdentity(transform) {
		if err := transformModelPositions(document, transform, request.CenterXZAtGround); err != nil {
			return nil, err
		}
	}
	if request.CompactBuffers {
		if err := compactModelBuffers(document); err != nil {
			return nil, err
		}
	} else if err := flattenModelBuffers(document); err != nil {
		return nil, err
	}
	if err := setModelBuffers(document); err != nil {
		return nil, err
	}
	return warnings, nil
}

func validateModelReferences(object map[string]json.RawMessage) error {
	accessors, err := modelArray(object, "accessors")
	if err != nil {
		return err
	}
	meshes, err := modelArray(object, "meshes")
	if err != nil {
		return err
	}
	materials, err := modelArray(object, "materials")
	if err != nil {
		return err
	}
	for meshIndex, mesh := range meshes {
		primitives, err := modelArray(mesh, "primitives")
		if err != nil {
			return fmt.Errorf("mesh %d: %w", meshIndex, err)
		}
		for primitiveIndex, primitive := range primitives {
			attributes, err := modelObject(primitive, "attributes")
			if err != nil {
				return fmt.Errorf("mesh %d primitive %d: %w", meshIndex, primitiveIndex, err)
			}
			for name, raw := range attributes {
				index, err := modelIndex(raw, len(accessors))
				if err != nil {
					return fmt.Errorf("mesh %d primitive %d attribute %s: %w", meshIndex, primitiveIndex, name, err)
				}
				attributes[name] = raw
				_ = index
			}
			if raw, ok := primitive["indices"]; ok {
				if _, err := modelIndex(raw, len(accessors)); err != nil {
					return fmt.Errorf("mesh %d primitive %d indices: %w", meshIndex, primitiveIndex, err)
				}
			}
			if raw, ok := primitive["material"]; ok {
				index, err := modelIndex(raw, len(materials))
				if err != nil {
					return fmt.Errorf("mesh %d primitive %d material: %w", meshIndex, primitiveIndex, err)
				}
				_ = index
			}
		}
	}
	return nil
}

func modelIndex(raw json.RawMessage, length int) (int, error) {
	var index int
	if err := json.Unmarshal(raw, &index); err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("index is out of range")
	}
	return index, nil
}

func applyMaterialNames(object map[string]json.RawMessage, names map[string]string) error {
	if len(names) == 0 {
		return nil
	}
	materials, err := modelArray(object, "materials")
	if err != nil {
		return err
	}
	for index, material := range materials {
		name, ok := names[modelStringOrEmpty(material, "name")]
		if !ok {
			continue
		}
		if name == "" {
			return fmt.Errorf("material name for material %d cannot be empty", index)
		}
		if err := setModelValue(material, "name", name); err != nil {
			return err
		}
	}
	return setModelArray(object, "materials", materials)
}

func modelStringOrEmpty(object map[string]json.RawMessage, key string) string {
	value, _ := modelString(object, key)
	return value
}

func deduplicateModelMaterials(object map[string]json.RawMessage) error {
	materials, err := modelArray(object, "materials")
	if err != nil {
		return err
	}
	remap := make([]int, len(materials))
	unique := make([]map[string]json.RawMessage, 0, len(materials))
	seen := make(map[string]int, len(materials))
	for index, material := range materials {
		data, err := json.Marshal(material)
		if err != nil {
			return fmt.Errorf("encode material %d: %w", index, err)
		}
		key := string(data)
		if previous, ok := seen[key]; ok {
			remap[index] = previous
			continue
		}
		remap[index] = len(unique)
		seen[key] = len(unique)
		unique = append(unique, material)
	}
	if len(unique) == len(materials) {
		return nil
	}
	meshes, err := modelArray(object, "meshes")
	if err != nil {
		return err
	}
	for _, mesh := range meshes {
		primitives, err := modelArray(mesh, "primitives")
		if err != nil {
			return err
		}
		for _, primitive := range primitives {
			if raw, ok := primitive["material"]; ok {
				index, err := modelIndex(raw, len(remap))
				if err != nil {
					return err
				}
				if err := setModelValue(primitive, "material", remap[index]); err != nil {
					return err
				}
			}
		}
		if err := setModelArray(mesh, "primitives", primitives); err != nil {
			return err
		}
	}
	if err := setModelArray(object, "meshes", meshes); err != nil {
		return err
	}
	return setModelArray(object, "materials", unique)
}

func removeModelAttributes(object map[string]json.RawMessage, names []string) int {
	if len(names) == 0 {
		return 0
	}
	requested := make(map[string]struct{}, len(names))
	for _, name := range names {
		requested[name] = struct{}{}
	}
	meshes, _ := modelArray(object, "meshes")
	removed := 0
	for _, mesh := range meshes {
		primitives, _ := modelArray(mesh, "primitives")
		for _, primitive := range primitives {
			attributes, _ := modelObject(primitive, "attributes")
			for name := range requested {
				if _, ok := attributes[name]; ok {
					delete(attributes, name)
					removed++
				}
			}
			_ = setModelValue(primitive, "attributes", attributes)
		}
		_ = setModelArray(mesh, "primitives", primitives)
	}
	_ = setModelArray(object, "meshes", meshes)
	return removed
}

func identityModelMatrix() modelMatrix {
	return modelMatrix{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func modelRootTransform(object map[string]json.RawMessage) (modelMatrix, bool, error) {
	scenes, err := modelArray(object, "scenes")
	if err != nil {
		return modelMatrix{}, false, err
	}
	nodes, err := modelArray(object, "nodes")
	if err != nil {
		return modelMatrix{}, false, err
	}
	if len(scenes) == 0 || len(nodes) == 0 {
		return identityModelMatrix(), false, nil
	}
	sceneIndex := 0
	if raw, ok := object["scene"]; ok {
		var parseErr error
		sceneIndex, parseErr = modelIndex(raw, len(scenes))
		if parseErr != nil {
			return modelMatrix{}, false, fmt.Errorf("scene: %w", parseErr)
		}
	}
	rootIndices, present, err := modelIntArray(scenes[sceneIndex], "nodes")
	if err != nil {
		return modelMatrix{}, false, err
	}
	if !present || len(rootIndices) == 0 {
		return identityModelMatrix(), false, nil
	}
	result := identityModelMatrix()
	for index, nodeIndex := range rootIndices {
		if nodeIndex < 0 || nodeIndex >= len(nodes) {
			return modelMatrix{}, false, fmt.Errorf("scene root node index %d is out of range", nodeIndex)
		}
		transform, err := modelNodeMatrix(nodes[nodeIndex])
		if err != nil {
			return modelMatrix{}, false, fmt.Errorf("root node %d: %w", nodeIndex, err)
		}
		if index == 0 {
			result = transform
		} else if !modelMatricesEqual(result, transform) {
			return modelMatrix{}, false, errors.New("static model roots must use one shared transform before baking")
		}
	}
	return result, true, nil
}

func modelNodeMatrix(node map[string]json.RawMessage) (modelMatrix, error) {
	if matrix, present, err := modelFloatArray(node, "matrix", 16); present {
		if err != nil {
			return modelMatrix{}, err
		}
		var result modelMatrix
		copy(result[:], matrix)
		return result, nil
	}
	translation := []float64{0, 0, 0}
	if value, present, err := modelFloatArray(node, "translation", 3); present {
		if err != nil {
			return modelMatrix{}, err
		}
		translation = value
	}
	rotation := []float64{0, 0, 0, 1}
	if value, present, err := modelFloatArray(node, "rotation", 4); present {
		if err != nil {
			return modelMatrix{}, err
		}
		rotation = value
	}
	scale := []float64{1, 1, 1}
	if value, present, err := modelFloatArray(node, "scale", 3); present {
		if err != nil {
			return modelMatrix{}, err
		}
		scale = value
	}
	x, y, z, w := rotation[0], rotation[1], rotation[2], rotation[3]
	result := identityModelMatrix()
	result[0] = (1 - 2*(y*y+z*z)) * scale[0]
	result[1] = (2 * (x*y + z*w)) * scale[0]
	result[2] = (2 * (x*z - y*w)) * scale[0]
	result[4] = (2 * (x*y - z*w)) * scale[1]
	result[5] = (1 - 2*(x*x+z*z)) * scale[1]
	result[6] = (2 * (y*z + x*w)) * scale[1]
	result[8] = (2 * (x*z + y*w)) * scale[2]
	result[9] = (2 * (y*z - x*w)) * scale[2]
	result[10] = (1 - 2*(x*x+y*y)) * scale[2]
	result[12], result[13], result[14] = translation[0], translation[1], translation[2]
	return result, nil
}

func clearModelRootTransforms(object map[string]json.RawMessage) {
	scenes, _ := modelArray(object, "scenes")
	nodes, _ := modelArray(object, "nodes")
	if len(scenes) == 0 || len(nodes) == 0 {
		return
	}
	sceneIndex := 0
	if raw, ok := object["scene"]; ok {
		if index, err := modelIndex(raw, len(scenes)); err == nil {
			sceneIndex = index
		}
	}
	rootIndices, _, _ := modelIntArray(scenes[sceneIndex], "nodes")
	for _, index := range rootIndices {
		if index < 0 || index >= len(nodes) {
			continue
		}
		for _, key := range []string{"matrix", "translation", "rotation", "scale"} {
			delete(nodes[index], key)
		}
	}
	_ = setModelArray(object, "nodes", nodes)
}

func modelAnimations(object map[string]json.RawMessage) []map[string]json.RawMessage {
	values, _ := modelArray(object, "animations")
	return values
}

func modelMatricesEqual(left, right modelMatrix) bool {
	for index := range left {
		if math.Abs(left[index]-right[index]) > 1e-12 {
			return false
		}
	}
	return true
}

func modelMatrixIsIdentity(value modelMatrix) bool {
	return modelMatricesEqual(value, identityModelMatrix())
}

func modelMatrixDeterminant(value modelMatrix) float64 {
	return value[0]*(value[5]*value[10]-value[6]*value[9]) -
		value[4]*(value[1]*value[10]-value[2]*value[9]) +
		value[8]*(value[1]*value[6]-value[2]*value[5])
}

func modelTransformPoint(matrix modelMatrix, x, y, z float64) (float64, float64, float64) {
	return matrix[0]*x + matrix[4]*y + matrix[8]*z + matrix[12],
		matrix[1]*x + matrix[5]*y + matrix[9]*z + matrix[13],
		matrix[2]*x + matrix[6]*y + matrix[10]*z + matrix[14]
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

func flattenModelBuffers(document *modelDocument) error {
	views, err := modelArray(document.Object, "bufferViews")
	if err != nil {
		return err
	}
	if len(document.Buffers) == 0 {
		if len(views) == 0 {
			return nil
		}
		return errors.New("model has bufferViews but no buffers")
	}
	offsets := make([]int, len(document.Buffers))
	var combined []byte
	for index, data := range document.Buffers {
		combined = append(combined, make([]byte, (4-len(combined)%4)%4)...)
		offsets[index] = len(combined)
		combined = append(combined, data...)
	}
	for index, view := range views {
		bufferIndex, present, err := modelInt(view, "buffer")
		if err != nil || !present || bufferIndex < 0 || bufferIndex >= len(offsets) {
			return fmt.Errorf("bufferView %d has an invalid buffer", index)
		}
		byteOffset, _, err := modelInt(view, "byteOffset")
		if err != nil || byteOffset < 0 {
			return fmt.Errorf("bufferView %d has an invalid byteOffset", index)
		}
		if err := setModelValue(view, "buffer", 0); err != nil {
			return err
		}
		if err := setModelValue(view, "byteOffset", offsets[bufferIndex]+byteOffset); err != nil {
			return err
		}
	}
	document.Buffers = [][]byte{combined}
	return setModelArray(document.Object, "bufferViews", views)
}

func compactModelBuffers(document *modelDocument) error {
	if err := flattenModelBuffers(document); err != nil {
		return err
	}
	if len(document.Buffers) == 0 {
		return nil
	}
	views, err := modelArray(document.Object, "bufferViews")
	if err != nil {
		return err
	}
	used := referencedModelBufferViews(document.Object)
	remap := make(map[int]int, len(used))
	compact := make([]map[string]json.RawMessage, 0, len(used))
	var data []byte
	for index, view := range views {
		if _, ok := used[index]; !ok {
			continue
		}
		byteOffset, _, offsetErr := modelInt(view, "byteOffset")
		byteLength, present, lengthErr := modelInt(view, "byteLength")
		if offsetErr != nil || lengthErr != nil || !present || byteOffset < 0 || byteLength < 0 || byteOffset > len(document.Buffers[0]) || byteLength > len(document.Buffers[0])-byteOffset {
			return fmt.Errorf("bufferView %d exceeds the model buffer", index)
		}
		data = append(data, make([]byte, (4-len(data)%4)%4)...)
		newView := cloneModelObject(view)
		if err := setModelValue(newView, "byteOffset", len(data)); err != nil {
			return err
		}
		data = append(data, document.Buffers[0][byteOffset:byteOffset+byteLength]...)
		remap[index] = len(compact)
		compact = append(compact, newView)
	}
	if err := remapModelBufferViewReferences(document.Object, remap); err != nil {
		return err
	}
	document.Buffers = [][]byte{data}
	return setModelArray(document.Object, "bufferViews", compact)
}

func referencedModelBufferViews(object map[string]json.RawMessage) map[int]struct{} {
	used := make(map[int]struct{})
	accessors, _ := modelArray(object, "accessors")
	for _, accessor := range accessors {
		if index, present, err := modelInt(accessor, "bufferView"); present && err == nil {
			used[index] = struct{}{}
		}
		if sparse, err := modelObject(accessor, "sparse"); err == nil {
			for _, key := range []string{"indices", "values"} {
				if nested, err := modelObject(sparse, key); err == nil {
					if index, present, err := modelInt(nested, "bufferView"); present && err == nil {
						used[index] = struct{}{}
					}
				}
			}
		}
	}
	images, _ := modelArray(object, "images")
	for _, image := range images {
		if index, present, err := modelInt(image, "bufferView"); present && err == nil {
			used[index] = struct{}{}
		}
	}
	return used
}

func cloneModelObject(object map[string]json.RawMessage) map[string]json.RawMessage {
	clone := make(map[string]json.RawMessage, len(object))
	for key, value := range object {
		clone[key] = append(json.RawMessage(nil), value...)
	}
	return clone
}

func remapModelBufferViewReferences(object map[string]json.RawMessage, remap map[int]int) error {
	accessors, err := modelArray(object, "accessors")
	if err != nil {
		return err
	}
	for _, accessor := range accessors {
		if err := remapModelIndex(accessor, "bufferView", remap); err != nil {
			return err
		}
		if sparse, err := modelObject(accessor, "sparse"); err == nil {
			for _, key := range []string{"indices", "values"} {
				if nested, err := modelObject(sparse, key); err == nil {
					if err := remapModelIndex(nested, "bufferView", remap); err != nil {
						return err
					}
				}
			}
		}
	}
	images, err := modelArray(object, "images")
	if err != nil {
		return err
	}
	for _, image := range images {
		if err := remapModelIndex(image, "bufferView", remap); err != nil {
			return err
		}
	}
	if err := setModelArray(object, "accessors", accessors); err != nil {
		return err
	}
	return setModelArray(object, "images", images)
}

func remapModelIndex(object map[string]json.RawMessage, key string, remap map[int]int) error {
	raw, ok := object[key]
	if !ok {
		return nil
	}
	index, err := modelIntValue(raw)
	if err != nil {
		return err
	}
	newIndex, ok := remap[index]
	if !ok {
		return fmt.Errorf("bufferView %d is not retained during compaction", index)
	}
	return setModelValue(object, key, newIndex)
}

func setModelBuffers(document *modelDocument) error {
	if len(document.Buffers) == 0 {
		return nil
	}
	entry := map[string]json.RawMessage{}
	if err := setModelValue(entry, "byteLength", len(document.Buffers[0])); err != nil {
		return err
	}
	if err := setModelDataURI(entry, "uri", document.Buffers[0]); err != nil {
		return err
	}
	return setModelArray(document.Object, "buffers", []map[string]json.RawMessage{entry})
}
