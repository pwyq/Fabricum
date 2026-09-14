package processing

import (
	"encoding/json"
	"errors"
	"fmt"
)

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
