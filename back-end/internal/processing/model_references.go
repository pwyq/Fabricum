package processing

import (
	"encoding/json"
	"fmt"
)

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
