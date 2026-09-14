package processing

import (
	"errors"
	"fmt"
)

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
