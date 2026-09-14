package processing

import (
	"errors"
	"fmt"
	"math"
)

func gltfPrimitiveCount(document glTFDocument, primitive gltfPrimitive) (int, error) {
	if primitive.Indices != nil {
		return gltfAccessorCount(document, *primitive.Indices)
	}
	position, ok := primitive.Attributes["POSITION"]
	if !ok {
		return 0, nil
	}
	return gltfAccessorCount(document, position)
}

func gltfAccessorCount(document glTFDocument, index int) (int, error) {
	if index < 0 || index >= len(document.Accessors) {
		return 0, fmt.Errorf("glTF accessor index %d is out of range", index)
	}
	if document.Accessors[index].Count < 0 {
		return 0, errors.New("glTF accessor count cannot be negative")
	}
	return document.Accessors[index].Count, nil
}

func trianglesForPrimitive(mode, count int) int {
	switch mode {
	case 4:
		return count / 3
	case 5, 6:
		if count < 3 {
			return 0
		}
		return count - 2
	default:
		return 0
	}
}

func gltfAccessorBounds(document glTFDocument, index int) (*Bounds, error) {
	if index < 0 || index >= len(document.Accessors) {
		return nil, fmt.Errorf("glTF POSITION accessor index %d is out of range", index)
	}
	accessor := document.Accessors[index]
	if len(accessor.Min) == 0 && len(accessor.Max) == 0 {
		return nil, nil
	}
	if len(accessor.Min) < 3 || len(accessor.Max) < 3 || accessor.Type != "VEC3" {
		return nil, errors.New("glTF POSITION accessor has invalid bounds")
	}
	result := &Bounds{}
	for axis := range 3 {
		result.Min[axis], result.Max[axis] = accessor.Min[axis], accessor.Max[axis]
		if math.IsNaN(result.Min[axis]) || math.IsNaN(result.Max[axis]) || math.IsInf(result.Min[axis], 0) || math.IsInf(result.Max[axis], 0) || result.Min[axis] > result.Max[axis] {
			return nil, errors.New("glTF POSITION accessor has invalid bounds")
		}
	}
	return result, nil
}

func mergeBounds(left, right *Bounds) *Bounds {
	if right == nil {
		return left
	}
	if left == nil {
		copy := *right
		return &copy
	}
	for axis := range 3 {
		left.Min[axis] = minFloat(left.Min[axis], right.Min[axis])
		left.Max[axis] = maxFloat(left.Max[axis], right.Max[axis])
	}
	return left
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
