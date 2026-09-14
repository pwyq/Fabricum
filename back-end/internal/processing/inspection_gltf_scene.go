package processing

import "fmt"

func gltfSceneBounds(document glTFDocument, meshBounds []*Bounds) (*Bounds, error) {
	if len(document.Nodes) == 0 {
		return mergeGltfMeshBounds(meshBounds), nil
	}
	instances, err := gltfMeshInstances(document)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return mergeGltfMeshBounds(meshBounds), nil
	}
	var bounds *Bounds
	for meshIndex, matrices := range instances {
		for _, matrix := range matrices {
			bounds = mergeBounds(bounds, transformGltfBounds(meshBounds[meshIndex], matrix))
		}
	}
	return bounds, nil
}

func mergeGltfMeshBounds(meshBounds []*Bounds) *Bounds {
	var bounds *Bounds
	for _, mesh := range meshBounds {
		bounds = mergeBounds(bounds, mesh)
	}
	return bounds
}

func gltfMeshInstances(document glTFDocument) (map[int][]modelMatrix, error) {
	roots, err := gltfSceneRoots(document)
	if err != nil {
		return nil, err
	}
	instances := make(map[int][]modelMatrix)
	visiting := make([]bool, len(document.Nodes))
	var visit func(int, modelMatrix) error
	visit = func(index int, parent modelMatrix) error {
		if index < 0 || index >= len(document.Nodes) {
			return fmt.Errorf("node index %d is out of range", index)
		}
		if visiting[index] {
			return fmt.Errorf("node %d contains a cycle", index)
		}
		visiting[index] = true
		defer func() { visiting[index] = false }()
		local, err := modelNodeMatrix(document.Nodes[index])
		if err != nil {
			return fmt.Errorf("node %d: %w", index, err)
		}
		world := multiplyModelMatrices(parent, local)
		if raw, present := document.Nodes[index]["mesh"]; present {
			meshIndex, err := modelIndex(raw, len(document.Meshes))
			if err != nil {
				return fmt.Errorf("node %d mesh: %w", index, err)
			}
			instances[meshIndex] = append(instances[meshIndex], world)
		}
		children, present, err := modelIntArray(document.Nodes[index], "children")
		if err != nil {
			return fmt.Errorf("node %d children: %w", index, err)
		}
		if !present {
			return nil
		}
		for _, child := range children {
			if err := visit(child, world); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := visit(root, identityModelMatrix()); err != nil {
			return nil, err
		}
	}
	return instances, nil
}

func gltfSceneRoots(document glTFDocument) ([]int, error) {
	if len(document.Scenes) > 0 {
		sceneIndex := 0
		if document.Scene != nil {
			sceneIndex = *document.Scene
		}
		if sceneIndex < 0 || sceneIndex >= len(document.Scenes) {
			return nil, fmt.Errorf("scene index %d is out of range", sceneIndex)
		}
		roots, present, err := modelIntArray(document.Scenes[sceneIndex], "nodes")
		if err != nil {
			return nil, err
		}
		if !present {
			return nil, nil
		}
		return roots, nil
	}
	parents := make([]bool, len(document.Nodes))
	for index, node := range document.Nodes {
		children, present, err := modelIntArray(node, "children")
		if err != nil {
			return nil, fmt.Errorf("node %d children: %w", index, err)
		}
		if !present {
			continue
		}
		for _, child := range children {
			if child < 0 || child >= len(document.Nodes) {
				return nil, fmt.Errorf("node %d child index %d is out of range", index, child)
			}
			parents[child] = true
		}
	}
	roots := make([]int, 0, len(document.Nodes))
	for index, parent := range parents {
		if !parent {
			roots = append(roots, index)
		}
	}
	if len(roots) == 0 {
		for index := range document.Nodes {
			roots = append(roots, index)
		}
	}
	return roots, nil
}

func multiplyModelMatrices(left, right modelMatrix) modelMatrix {
	var result modelMatrix
	for column := 0; column < 4; column++ {
		for row := 0; row < 4; row++ {
			for index := 0; index < 4; index++ {
				result[column*4+row] += left[index*4+row] * right[column*4+index]
			}
		}
	}
	return result
}

func transformGltfBounds(bounds *Bounds, matrix modelMatrix) *Bounds {
	if bounds == nil {
		return nil
	}
	var result *Bounds
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			for z := 0; z < 2; z++ {
				pointX, pointY, pointZ := bounds.Min[0], bounds.Min[1], bounds.Min[2]
				if x == 1 {
					pointX = bounds.Max[0]
				}
				if y == 1 {
					pointY = bounds.Max[1]
				}
				if z == 1 {
					pointZ = bounds.Max[2]
				}
				pointX, pointY, pointZ = modelTransformPoint(matrix, pointX, pointY, pointZ)
				point := &Bounds{Min: [3]float64{pointX, pointY, pointZ}, Max: [3]float64{pointX, pointY, pointZ}}
				result = mergeBounds(result, point)
			}
		}
	}
	return result
}
