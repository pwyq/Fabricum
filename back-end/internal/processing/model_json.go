package processing

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
)

func modelArray(object map[string]json.RawMessage, key string) ([]map[string]json.RawMessage, error) {
	raw, ok := object[key]
	if !ok {
		return nil, nil
	}
	var values []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%s must be an array of objects: %w", key, err)
	}
	for index, value := range values {
		if value == nil {
			return nil, fmt.Errorf("%s[%d] must be an object", key, index)
		}
	}
	return values, nil
}

func setModelArray(object map[string]json.RawMessage, key string, values []map[string]json.RawMessage) error {
	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	object[key] = data
	return nil
}

func modelObject(object map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	raw, ok := object[key]
	if !ok {
		return nil, fmt.Errorf("missing %s object", key)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return value, nil
}

func modelString(object map[string]json.RawMessage, key string) (string, error) {
	raw, ok := object[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return value, nil
}

func modelInt(object map[string]json.RawMessage, key string) (int, bool, error) {
	raw, ok := object[key]
	if !ok {
		return 0, false, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, true, fmt.Errorf("%s must be an integer", key)
	}
	return value, true, nil
}

func modelFloatArray(object map[string]json.RawMessage, key string, length int) ([]float64, bool, error) {
	raw, ok := object[key]
	if !ok {
		return nil, false, nil
	}
	var value []float64
	if err := json.Unmarshal(raw, &value); err != nil || len(value) != length {
		return nil, true, fmt.Errorf("%s must contain %d numbers", key, length)
	}
	return value, true, nil
}

func modelIntArray(object map[string]json.RawMessage, key string) ([]int, bool, error) {
	raw, ok := object[key]
	if !ok {
		return nil, false, nil
	}
	var value []int
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, true, fmt.Errorf("%s must contain integers", key)
	}
	return value, true, nil
}

func setModelValue(object map[string]json.RawMessage, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	object[key] = data
	return nil
}

func setModelDataURI(object map[string]json.RawMessage, key string, data []byte) error {
	return setModelValue(object, key, "data:application/octet-stream;base64,"+base64.StdEncoding.EncodeToString(data))
}

func modelRawNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
