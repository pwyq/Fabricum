package processing

import (
	"encoding/json"
	"fmt"
)

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
