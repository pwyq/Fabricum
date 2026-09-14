package processing

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

type modelMatrix [16]float64

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
