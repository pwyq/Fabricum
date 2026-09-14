package compatibility

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	fabricum "fabricum/back-end"
)

// Run generates disposable project-neutral fixtures and exercises every
// current processing entry point. Native tools are discovered by Fabricum;
// this package does not invoke Node, a browser, or Sharp.
func Run(ctx context.Context, output io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	root, err := os.MkdirTemp("", "fabricum-compatibility-")
	if err != nil {
		return fmt.Errorf("create compatibility workspace: %w", err)
	}
	defer os.RemoveAll(root)
	fixture, err := createFixtures(root)
	if err != nil {
		return err
	}
	imageExports, err := runImageWorkflow(ctx, fixture)
	if err != nil {
		return err
	}
	sprites, err := runSpriteWorkflow(ctx, fixture)
	if err != nil {
		return err
	}
	materialSet, err := runTextureWorkflow(ctx, fixture)
	if err != nil {
		return err
	}
	models, err := runModelWorkflow(ctx, fixture)
	if err != nil {
		return err
	}
	inspection, err := runInspection(fixture, imageExports, sprites, materialSet, models)
	if err != nil {
		return err
	}
	receipt := Receipt{
		SchemaVersion: ReceiptSchemaVersion, Processor: "fabricum/" + fabricum.Version,
		FixturePolicy: "synthetic fixtures are generated in a disposable temporary directory",
		ImageExports:  imageExports, Sprites: sprites, MaterialSet: materialSet, Models: models, Inspection: inspection,
	}
	if err := json.NewEncoder(output).Encode(receipt); err != nil {
		return fmt.Errorf("write compatibility receipt: %w", err)
	}
	return nil
}

func runInspection(fixture fixtures, imageExports []fabricum.ExportReceipt, sprites fabricum.TransformReceipt, material fabricum.TextureReceipt, models []fabricum.ModelReceipt) (fabricum.InspectionReport, error) {
	paths := []string{fixture.Source, fixture.Transparent, fixture.JPEG, fixture.GIF, fixture.Normal, fixture.AO, fixture.Roughness, fixture.Model, fixture.ModelBin}
	for _, receipt := range imageExports {
		for _, output := range receipt.Outputs {
			paths = append(paths, output.Path)
		}
	}
	for _, output := range sprites.Outputs {
		paths = append(paths, output.Path)
	}
	for _, output := range material.Outputs {
		paths = append(paths, output.Path)
	}
	for _, receipt := range models {
		for _, output := range receipt.Outputs {
			paths = append(paths, output.Path)
		}
	}
	paths = uniquePaths(paths)
	var reportBytes bytes.Buffer
	if err := fabricum.Inspect(paths, &reportBytes); err != nil {
		return fabricum.InspectionReport{}, err
	}
	if strings.Contains(strings.ToLower(reportBytes.String()), "sha256") {
		return fabricum.InspectionReport{}, fmt.Errorf("inspection report exposed a content hash")
	}
	var report fabricum.InspectionReport
	if err := readJSON(reportBytes.Bytes(), &report); err != nil {
		return fabricum.InspectionReport{}, err
	}
	if report.SchemaVersion != fabricum.InspectionSchemaVersion || len(report.Assets) != len(paths) || report.Processor == "" {
		return fabricum.InspectionReport{}, fmt.Errorf("inspection returned an incomplete report")
	}
	for _, asset := range report.Assets {
		if asset.Error != "" || asset.Format == "" || asset.Bytes <= 0 {
			return fabricum.InspectionReport{}, fmt.Errorf("inspection failed for %s: %s", asset.Path, asset.Error)
		}
	}
	return report, nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		key := filepath.Clean(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, path)
	}
	return result
}
