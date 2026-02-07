package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/omarkamali/semango/internal/ingest"
	"github.com/omarkamali/semango/internal/util"
	"github.com/spf13/cobra"
)

const modelMetadataFile = ".semango-model.json"

type modelEntry struct {
	ID    string
	Alias string
}

type modelMetadata struct {
	ID    string `json:"id"`
	Alias string `json:"alias,omitempty"`
}

func NewModelsCmd() *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Manage local ONNX embedding models",
		Long:  "List, search, and download compatible ONNX embedding models.",
	}

	modelsSearchCmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for compatible ONNX models online",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}

			fmt.Printf("Searching for models matching '%s'...\n", query)
			results, err := ingest.SearchModelsOnline(query)
			if err != nil {
				return util.WrapError(err, "Failed to search models")
			}

			cacheDir, err := ingest.DefaultModelCacheDir()
			if err != nil {
				return util.WrapError(err, "Failed to determine model cache directory")
			}

			if len(results) == 0 {
				fmt.Println("No models found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL ID\tALIAS\tSIZE\tVRAM\tSTATUS")
			fmt.Fprintln(w, "--------\t-----\t----\t----\t------")

			for _, info := range results {
				installed := isModelInstalled(cacheDir, info.ID)
				status := ""
				if installed {
					status = "INSTALLED"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", info.ID, info.Alias, info.Size, info.VRAM, status)
				if info.Description != "" {
					fmt.Fprintf(w, "  └─ %s\t\t\t\t\n", info.Description)
				}
			}
			w.Flush()

			fmt.Printf("\nTo download a model, run: semango models download <alias-or-id>\n")
			return nil
		},
	}

	modelsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List locally installed ONNX models",
		RunE: func(cmd *cobra.Command, args []string) error {
			cacheDir, err := ingest.DefaultModelCacheDir()
			if err != nil {
				return util.WrapError(err, "Failed to determine model cache directory")
			}

			fmt.Printf("Model cache: %s\n", cacheDir)
			localModels := []ingest.ModelInfo{}
			walkErr := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				if d.IsDir() {
					return nil
				}
				if d.Name() != modelMetadataFile {
					return nil
				}
				meta, err := readModelMetadata(filepath.Dir(path))
				if err == nil && meta.ID != "" {
					localModels = append(localModels, ingest.GetModelMetadata(meta.ID))
				}
				return nil
			})
			if walkErr != nil {
				return util.WrapError(walkErr, "Failed to read model cache directory")
			}

			if len(localModels) == 0 {
				fmt.Println("No local models installed.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL ID\tALIAS\tSIZE\tVRAM")
			fmt.Fprintln(w, "--------\t-----\t----\t----")

			for _, model := range localModels {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", model.ID, model.Alias, model.Size, model.VRAM)
			}
			w.Flush()

			return nil
		},
	}

	modelsDownloadCmd := &cobra.Command{
		Use:   "download [model-id-or-alias]",
		Short: "Download a compatible ONNX model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			cacheDir, err := ingest.DefaultModelCacheDir()
			if err != nil {
				return util.WrapError(err, "Failed to determine model cache directory")
			}

			// Resolve metadata
			info := ingest.GetModelMetadata(target)
			if !strings.HasPrefix(info.ID, "onnx-models/") && !strings.Contains(info.ID, "/") {
				results, err := ingest.SearchModelsOnline(target)
				if err == nil && len(results) > 0 {
					for _, r := range results {
						if r.Alias == target || r.ID == target {
							info = r
							break
						}
					}
				}
			}

			force, _ := cmd.Flags().GetBool("force")
			modelDir := modelCachePath(cacheDir, info.ID)
			if !force && isModelInstalled(cacheDir, info.ID) {
				fmt.Printf("Model already installed at %s\n", modelDir)
				return nil
			}

			if force {
				_ = os.RemoveAll(modelDir)
				_ = os.RemoveAll(legacyModelCachePath(cacheDir, info.ID))
			}

			fmt.Printf("Downloading %s...\n", info.ID)
			if _, err := ingest.DownloadONNXModel(info.ID, cacheDir); err != nil {
				return util.WrapError(err, "Failed to download model")
			}

			if err := writeModelMetadata(modelDir, modelEntry{ID: info.ID, Alias: info.Alias}); err != nil {
				return util.WrapError(err, "Failed to write model metadata")
			}

			fmt.Printf("Successfully downloaded %s to %s\n", info.ID, modelDir)
			return nil
		},
	}

	modelsDeleteCmd := &cobra.Command{
		Use:   "delete [model-id-or-alias]",
		Short: "Delete a locally installed ONNX model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			cacheDir, err := ingest.DefaultModelCacheDir()
			if err != nil {
				return util.WrapError(err, "Failed to determine model cache directory")
			}

			modelDir := ""
			if strings.HasPrefix(target, "onnx-models/") {
				modelDir = modelCachePath(cacheDir, target)
			} else {
				info := ingest.GetModelMetadata(target)
				if strings.HasPrefix(info.ID, "onnx-models/") {
					modelDir = modelCachePath(cacheDir, info.ID)
				} else {
					modelDir = filepath.Join(cacheDir, target)
				}
			}

			if info, err := os.Stat(modelDir); err != nil || !info.IsDir() {
				legacyDir := modelDir
				if strings.Contains(target, "/") {
					legacyDir = legacyModelCachePath(cacheDir, target)
				} else {
					info := ingest.GetModelMetadata(target)
					legacyDir = legacyModelCachePath(cacheDir, info.ID)
				}
				modelDir = legacyDir
			}

			info, statErr := os.Stat(modelDir)
			if statErr != nil || !info.IsDir() {
				return util.NewError(fmt.Sprintf("Model not found in cache: %s", target))
			}

			if err := os.RemoveAll(modelDir); err != nil {
				return util.WrapError(err, "Failed to delete model")
			}

			fmt.Printf("Deleted model at %s\n", modelDir)
			return nil
		},
	}

	modelsDownloadCmd.Flags().Bool("force", false, "Re-download even if the model is already installed")
	modelsCmd.AddCommand(modelsSearchCmd, modelsListCmd, modelsDownloadCmd, modelsDeleteCmd)
	return modelsCmd
}

func isModelInstalled(cacheDir, modelID string) bool {
	modelDir := modelCachePath(cacheDir, modelID)
	info, err := os.Stat(modelDir)
	if err == nil && info.IsDir() {
		return true
	}
	legacyDir := legacyModelCachePath(cacheDir, modelID)
	info, err = os.Stat(legacyDir)
	return err == nil && info.IsDir()
}

func modelCachePath(cacheDir, modelID string) string {
	return filepath.Join(cacheDir, filepath.FromSlash(modelID))
}

func legacyModelCachePath(cacheDir, modelID string) string {
	return filepath.Join(cacheDir, strings.ReplaceAll(modelID, "/", "_"))
}

func readModelMetadata(modelDir string) (modelMetadata, error) {
	metaPath := filepath.Join(modelDir, modelMetadataFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return modelMetadata{}, err
	}
	var meta modelMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return modelMetadata{}, err
	}
	return meta, nil
}

func writeModelMetadata(modelDir string, entry modelEntry) error {
	meta := modelMetadata(entry)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metaPath := filepath.Join(modelDir, modelMetadataFile)
	return os.WriteFile(metaPath, data, 0644)
}
