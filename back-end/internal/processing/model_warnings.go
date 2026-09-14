package processing

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toolWarnings(data []byte) []string {
	var warnings []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "warning") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

func unsupportedModelWarnings(object map[string]json.RawMessage) []string {
	known := map[string]struct{}{
		"KHR_lights_punctual": {}, "KHR_materials_anisotropy": {}, "KHR_materials_clearcoat": {},
		"KHR_materials_diffuse_transmission": {}, "KHR_materials_dispersion": {}, "KHR_materials_emissive_strength": {},
		"KHR_materials_ior": {}, "KHR_materials_iridescence": {}, "KHR_materials_pbrSpecularGlossiness": {},
		"KHR_materials_sheen": {}, "KHR_materials_specular": {}, "KHR_materials_transmission": {},
		"KHR_materials_unlit": {}, "KHR_materials_variants": {}, "KHR_materials_volume": {},
		"KHR_mesh_quantization": {}, "KHR_meshopt_compression": {}, "KHR_texture_basisu": {},
		"KHR_texture_transform": {}, "EXT_mesh_gpu_instancing": {}, "EXT_meshopt_compression": {},
		"EXT_texture_webp": {},
	}
	var extensions []string
	for _, key := range []string{"extensionsUsed", "extensionsRequired"} {
		raw, ok := object[key]
		if !ok {
			continue
		}
		var values []string
		if json.Unmarshal(raw, &values) == nil {
			extensions = append(extensions, values...)
		}
	}
	var warnings []string
	seen := make(map[string]struct{})
	for _, extension := range extensions {
		if _, ok := known[extension]; !ok {
			if _, duplicate := seen[extension]; duplicate {
				continue
			}
			seen[extension] = struct{}{}
			warnings = append(warnings, fmt.Sprintf("unsupported extension %s may be discarded by gltfpack", extension))
		}
	}
	return warnings
}
