package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	ortlib "github.com/omarkamali/semango/internal/onnxruntime"
	"github.com/omarkamali/semango/internal/util"
	"github.com/omarkamali/semango/pkg/semango"
	"github.com/yalue/onnxruntime_go"
)

// LocalEmbedder implements the Embedder interface using local ONNX models.
// It supports sentence transformer models from the onnx-models organization on Hugging Face.
type LocalEmbedder struct {
	modelDir       string
	modelOnnxPath  string
	dimension      int
	maxLength      int
	batchSize      int
	tokenizer      *Tokenizer
	session        *onnxruntime_go.DynamicAdvancedSession
	inputNames     []string
	poolingConfig  *PoolingConfig
	outputName     string // Cached output name for the ONNX model
	outputIsPooled bool   // Whether the output is already pooled (sentence-level)
	enableGPU      bool   // Whether GPU is enabled
	mu             sync.RWMutex
}

// LocalEmbedderConfig holds configuration for the local embedder.
type LocalEmbedderConfig struct {
	ModelPath  string // Path to the model directory or onnx-models model name
	CacheDir   string // Directory to cache downloaded models
	BatchSize  int    // Batch size for inference
	MaxLength  int    // Maximum sequence length
	ModelName  string // Specific model name (e.g., "all-MiniLM-L6-v2-onnx")
	OutputName string // Manually specified output name
	EnableGPU  bool   // Whether to enable GPU acceleration
}

// Tokenizer handles text tokenization for sentence transformers.
type Tokenizer struct {
	vocab         map[string]int
	vocabReverse  map[int]string
	specialTokens map[string]int
	maxLength     int
	padToken      string
	unkToken      string
	clsToken      string
	sepToken      string
	maskToken     string
	doLowerCase   bool
}

// PoolingConfig defines how to pool token embeddings into sentence embeddings.
type PoolingConfig struct {
	WordEmbeddingDimension int  `json:"word_embedding_dimension"`
	PoolingModeCLSToken    bool `json:"pooling_mode_cls_token"`
	PoolingModeMeanTokens  bool `json:"pooling_mode_mean_tokens"`
	PoolingModeMaxTokens   bool `json:"pooling_mode_max_tokens"`
	PoolingModeMeanSqrtLen bool `json:"pooling_mode_mean_sqrt_len_tokens"`
	IncludePrompt          bool `json:"include_prompt"`
}

// TokenizerConfig holds tokenizer configuration.
type TokenizerConfig struct {
	VocabSize     int               `json:"vocab_size"`
	MaxPosition   int               `json:"max_position_embeddings"`
	SpecialTokens map[string]string `json:"special_tokens_map"`
	DoLowerCase   bool              `json:"do_lower_case"`
}

// NewLocalEmbedder creates a new local embedder instance.
func NewLocalEmbedder(config LocalEmbedderConfig) (*LocalEmbedder, error) {
	if config.ModelPath == "" {
		return nil, fmt.Errorf("model path is required")
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 32 // Default batch size
	}
	if config.MaxLength <= 0 {
		config.MaxLength = 512 // Default max length
	}
	if config.CacheDir == "" {
		homeDir, _ := os.UserHomeDir()
		config.CacheDir = filepath.Join(homeDir, ".cache", "semango", "models")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	embedder := &LocalEmbedder{
		batchSize: config.BatchSize,
		maxLength: config.MaxLength,
		enableGPU: config.EnableGPU,
	}

	modelDir, modelOnnxPath, err := embedder.resolveModelLocation(config.ModelPath, config.CacheDir)
	if err != nil {
		return nil, err
	}

	embedder.modelDir = modelDir
	embedder.modelOnnxPath = modelOnnxPath

	// Load tokenizer
	tokenizer, err := embedder.loadTokenizer(modelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %w", err)
	}
	embedder.tokenizer = tokenizer

	// Load pooling configuration
	poolingConfig, err := embedder.loadPoolingConfig(modelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load pooling config: %w", err)
	}
	embedder.poolingConfig = poolingConfig
	embedder.dimension = poolingConfig.WordEmbeddingDimension

	// Initialize ONNX Runtime environment if not already done
	if !onnxruntime_go.IsInitialized() {
		libPath, err := ortlib.EnsureSharedLibrary()
		if err != nil {
			return nil, fmt.Errorf("failed to prepare onnxruntime library: %w", err)
		}
		onnxruntime_go.SetSharedLibraryPath(libPath)

		err = onnxruntime_go.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
		}
	}

	// Detect the correct output name for this ONNX model
	outputName, err := embedder.detectOutputName(modelOnnxPath, config.OutputName)
	if err != nil {
		return nil, fmt.Errorf("failed to detect output name: %w", err)
	}
	embedder.outputName = outputName

	// Initialize ONNX session
	session, err := embedder.initONNXSession(modelOnnxPath, outputName)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX session: %w", err)
	}
	embedder.session = session

	return embedder, nil
}

// isLocalPath checks if the given path is a local file system path.

func isLocalPath(path string) bool {
	if path == "" {
		return false
	}
	// Absolute or explicit relative paths are always local
	if filepath.IsAbs(path) || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	// Direct ONNX file references are local
	if strings.HasSuffix(strings.ToLower(path), ".onnx") {
		return true
	}
	// No slash => local name within cache dir
	if !strings.Contains(path, "/") {
		return true
	}
	// Has slash and is not an ONNX file => treat as Hugging Face model ID
	return false
}

func (le *LocalEmbedder) resolveModelLocation(modelRef, cacheDir string) (string, string, error) {
	if modelRef == "" {
		return "", "", fmt.Errorf("model path is required")
	}
	if cacheDir == "" {
		return "", "", fmt.Errorf("cache directory is required")
	}

	if isLocalPath(modelRef) {
		localPath := modelRef
		if !filepath.IsAbs(localPath) && !strings.HasPrefix(localPath, "./") && !strings.HasPrefix(localPath, "../") {
			localPath = filepath.Join(cacheDir, filepath.FromSlash(localPath))
		}
		return resolveLocalModelPath(localPath)
	}

	modelDir := filepath.Join(cacheDir, filepath.FromSlash(modelRef))
	if onnxPath, ok := isValidModelDir(modelDir); ok {
		return modelDir, onnxPath, nil
	}

	// Legacy directory check (underscore instead of slashes)
	legacyModelDir := filepath.Join(cacheDir, strings.ReplaceAll(modelRef, "/", "_"))
	if onnxPath, ok := isValidModelDir(legacyModelDir); ok {
		return legacyModelDir, onnxPath, nil
	}

	repoFiles, err := listHuggingFaceRepoFiles(modelRef)
	if err != nil {
		return "", "", fmt.Errorf("failed to query Hugging Face repo: %w", err)
	}
	onnxFiles := filterOnnxFiles(repoFiles)
	if len(onnxFiles) == 0 {
		return "", "", fmt.Errorf("no .onnx files found in Hugging Face repo: %s", modelRef)
	}

	// If directory exists but is invalid, we might want to clean it up or overwrite
	// For now, we'll just let MkdirAll and downloadHuggingFaceFiles handle it.
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create model directory: %w", err)
	}

	if err := le.downloadHuggingFaceFiles(modelRef, modelDir, repoFiles); err != nil {
		return "", "", fmt.Errorf("failed to download model files: %w", err)
	}

	selectedOnnx := selectOnnxFile(onnxFiles)
	onnxPath := filepath.Join(modelDir, selectedOnnx)
	if _, err := os.Stat(onnxPath); err != nil {
		return "", "", fmt.Errorf("ONNX model file not found after download: %s", onnxPath)
	}

	return modelDir, onnxPath, nil
}

func isValidModelDir(dir string) (string, bool) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", false
	}

	onnxPath, err := findOnnxFileInDir(dir)
	if err != nil {
		return "", false
	}

	// Check for tokenizer.json or vocab.txt
	tokenizerJson := filepath.Join(dir, "tokenizer.json")
	vocabTxt := filepath.Join(dir, "vocab.txt")

	hasTokenizer := false
	if info, err := os.Stat(tokenizerJson); err == nil && info.Size() > 0 {
		hasTokenizer = true
	} else if info, err := os.Stat(vocabTxt); err == nil && info.Size() > 0 {
		hasTokenizer = true
	}

	if !hasTokenizer {
		return "", false
	}

	return onnxPath, true
}

func resolveLocalModelPath(localPath string) (string, string, error) {
	if strings.HasSuffix(strings.ToLower(localPath), ".onnx") {
		if _, err := os.Stat(localPath); err != nil {
			return "", "", fmt.Errorf("ONNX model file not found: %s", localPath)
		}
		return filepath.Dir(localPath), localPath, nil
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return "", "", fmt.Errorf("model path not found: %s", localPath)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("model path is not a directory: %s", localPath)
	}

	onnxPath, err := findOnnxFileInDir(localPath)
	if err != nil {
		return "", "", err
	}

	return localPath, onnxPath, nil
}

func findOnnxFileInDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read model directory: %w", err)
	}

	preferred := ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "model.onnx") {
			preferred = name
			break
		}
	}
	if preferred != "" {
		return filepath.Join(dir, preferred), nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".onnx") {
			return filepath.Join(dir, name), nil
		}
	}

	return "", fmt.Errorf("no .onnx files found in model directory: %s", dir)
}

// downloadONNXModel downloads a model from onnx-models organization on Hugging Face Hub.
func (le *LocalEmbedder) downloadONNXModel(modelName, cacheDir string) (string, error) {
	// Handle both "model-name-onnx" and "onnx-models/model-name-onnx" formats
	var fullModelName string
	if strings.HasPrefix(modelName, "onnx-models/") {
		fullModelName = modelName
	} else {
		fullModelName = "onnx-models/" + modelName
	}

	modelDir := filepath.Join(cacheDir, filepath.FromSlash(fullModelName))

	// Check if model already exists
	if info, err := os.Stat(modelDir); err == nil && info.IsDir() {
		return modelDir, nil
	}

	repoFiles, err := listHuggingFaceRepoFiles(fullModelName)
	if err != nil {
		return "", fmt.Errorf("failed to query Hugging Face repo: %w", err)
	}
	if len(filterOnnxFiles(repoFiles)) == 0 {
		return "", fmt.Errorf("no .onnx files found in Hugging Face repo: %s", fullModelName)
	}

	// Create model directory
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	if err := le.downloadHuggingFaceFiles(fullModelName, modelDir, repoFiles); err != nil {
		return "", err
	}

	return modelDir, nil
}

// downloadFile downloads a file from URL to local path.
func (le *LocalEmbedder) downloadFile(url, localPath, description string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: status %d", url, resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer out.Close()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	return err
}

type hfModelResponse struct {
	Siblings []struct {
		RFilename string `json:"rfilename"`
	} `json:"siblings"`
}

func listHuggingFaceRepoFiles(modelID string) ([]string, error) {
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s", modelID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query model metadata: status %d", resp.StatusCode)
	}

	var parsed hfModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode model metadata: %w", err)
	}

	files := make([]string, 0, len(parsed.Siblings))
	for _, s := range parsed.Siblings {
		if s.RFilename != "" {
			files = append(files, s.RFilename)
		}
	}

	return files, nil
}

func filterOnnxFiles(files []string) []string {
	onnx := []string{}
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".onnx") {
			onnx = append(onnx, f)
		}
	}
	return onnx
}

func selectOnnxFile(onnxFiles []string) string {
	for _, f := range onnxFiles {
		if strings.EqualFold(filepath.Base(f), "model.onnx") {
			return f
		}
	}
	return onnxFiles[0]
}

func (le *LocalEmbedder) downloadHuggingFaceFiles(modelID, modelDir string, repoFiles []string) error {
	standardFiles := []string{
		"config.json",
		"tokenizer.json",
		"tokenizer_config.json",
		"vocab.txt",
		"1_Pooling/config.json",
		"special_tokens_map.json",
	}

	repoSet := make(map[string]struct{}, len(repoFiles))
	for _, f := range repoFiles {
		repoSet[f] = struct{}{}
	}

	filesToDownload := make(map[string]struct{})
	for _, f := range standardFiles {
		if _, ok := repoSet[f]; ok {
			filesToDownload[f] = struct{}{}
		}
	}
	for _, f := range repoFiles {
		if strings.HasSuffix(strings.ToLower(f), ".onnx") {
			filesToDownload[f] = struct{}{}
		}
	}

	baseURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main", modelID)

	for file := range filesToDownload {
		url := fmt.Sprintf("%s/%s", baseURL, file)
		localPath := filepath.Join(modelDir, filepath.FromSlash(file))

		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", file, err)
		}

		desc := fmt.Sprintf("Downloading %s ", filepath.Base(file))
		if err := le.downloadFile(url, localPath, desc); err != nil {
			return err
		}
	}

	return nil
}

// loadTokenizer loads the tokenizer from the model directory.
func (le *LocalEmbedder) loadTokenizer(modelDir string) (*Tokenizer, error) {
	// Try to load tokenizer.json first (modern format)
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	if _, err := os.Stat(tokenizerPath); err == nil {
		return le.loadTokenizerJSON(tokenizerPath)
	}

	// Fallback to vocab.txt (older format)
	vocabPath := filepath.Join(modelDir, "vocab.txt")
	if _, err := os.Stat(vocabPath); err == nil {
		return le.loadTokenizerVocab(vocabPath)
	}

	return nil, fmt.Errorf("no tokenizer files found in %s", modelDir)
}

// loadTokenizerJSON loads tokenizer from tokenizer.json.
func (le *LocalEmbedder) loadTokenizerJSON(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tokenizerData struct {
		Model struct {
			Vocab map[string]int `json:"vocab"`
		} `json:"model"`
		AddedTokens []struct {
			ID      int    `json:"id"`
			Content string `json:"content"`
			Special bool   `json:"special"`
		} `json:"added_tokens"`
		Truncation struct {
			MaxLength int `json:"max_length"`
		} `json:"truncation"`
		Normalizer struct {
			Lowercase bool `json:"lowercase"`
		} `json:"normalizer"`
	}

	if err := json.Unmarshal(data, &tokenizerData); err != nil {
		return nil, err
	}

	tokenizer := &Tokenizer{
		vocab:         tokenizerData.Model.Vocab,
		vocabReverse:  make(map[int]string),
		specialTokens: make(map[string]int),
		maxLength:     tokenizerData.Truncation.MaxLength,
		padToken:      "[PAD]",
		unkToken:      "[UNK]",
		clsToken:      "[CLS]",
		sepToken:      "[SEP]",
		maskToken:     "[MASK]",
		doLowerCase:   tokenizerData.Normalizer.Lowercase,
	}

	// Build reverse vocab
	for token, id := range tokenizer.vocab {
		tokenizer.vocabReverse[id] = token
	}

	// Add special tokens
	for _, token := range tokenizerData.AddedTokens {
		if token.Special {
			tokenizer.specialTokens[token.Content] = token.ID
		}
	}

	return tokenizer, nil
}

// loadTokenizerVocab loads tokenizer from vocab.txt.
func (le *LocalEmbedder) loadTokenizerVocab(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	vocab := make(map[string]int)
	vocabReverse := make(map[int]string)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			vocab[line] = i
			vocabReverse[i] = line
		}
	}

	tokenizer := &Tokenizer{
		vocab:         vocab,
		vocabReverse:  vocabReverse,
		specialTokens: make(map[string]int),
		maxLength:     le.maxLength,
		padToken:      "[PAD]",
		unkToken:      "[UNK]",
		clsToken:      "[CLS]",
		sepToken:      "[SEP]",
		maskToken:     "[MASK]",
		doLowerCase:   true, // Default for most models
	}

	// Set special token IDs
	for token, id := range vocab {
		if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") {
			tokenizer.specialTokens[token] = id
		}
	}

	return tokenizer, nil
}

// loadPoolingConfig loads pooling configuration.
func (le *LocalEmbedder) loadPoolingConfig(modelDir string) (*PoolingConfig, error) {
	poolingPath := filepath.Join(modelDir, "1_Pooling", "config.json")
	if _, err := os.Stat(poolingPath); os.IsNotExist(err) {
		// Default pooling configuration
		return &PoolingConfig{
			WordEmbeddingDimension: 384, // Default dimension
			PoolingModeMeanTokens:  true,
			IncludePrompt:          true,
		}, nil
	}

	data, err := os.ReadFile(poolingPath)
	if err != nil {
		return nil, err
	}

	var config PoolingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// detectOutputName detects the correct output name for the ONNX model.
func (le *LocalEmbedder) detectOutputName(modelOnnxPath string, manualOutputName string) (string, error) {

	// Get model info to determine available output names
	_, outputs, err := onnxruntime_go.GetInputOutputInfo(modelOnnxPath)
	if err != nil {
		return "", fmt.Errorf("failed to get model info for %s: %w", modelOnnxPath, err)
	}

	availableOutputs := make([]string, 0, len(outputs))
	for _, o := range outputs {
		availableOutputs = append(availableOutputs, o.Name)
	}

	if manualOutputName != "" {
		for _, name := range availableOutputs {
			if name == manualOutputName {
				return name, nil
			}
		}
		return "", fmt.Errorf("manually specified output layer '%s' not found in model %s.\n\n"+
			"Available output layers: %v\n\n"+
			"To fix this:\n"+
			"1. Inspect your model architecture at https://netron.app by uploading your model.onnx file.\n"+
			"2. Find the Name of the output node you wish to use (usually the last Layer or a pooling Layer).\n"+
			"3. Update your semango.yml: embedding.onnx_output_name: \"<name>\"",
			manualOutputName, modelOnnxPath, availableOutputs)
	}

	// Priority list for sentence-level or good token-level embeddings.
	// We avoid "token_embeddings" if anything else is available as per user request.
	priorities := []string{
		"sentence_embedding",
		"pooler_output",
		"last_hidden_state",
		"output",
		"embeddings",
		"hidden_states",
	}

	for _, p := range priorities {
		for _, name := range availableOutputs {
			if name == p {
				slog.Debug("Detected ONNX output name", "name", name)
				return name, nil
			}
		}
	}

	// If none of the preferred ones exist, pick the first one from the model that isn't token_embeddings
	if len(availableOutputs) > 0 {
		for _, name := range availableOutputs {
			if name != "token_embeddings" {
				slog.Debug("Detected non-priority ONNX output name", "name", name)
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("could not automatically detect a suitable output layer for the ONNX model (found: %v).\n\n"+
		"Note: 'token_embeddings' was found but is insufficient as it requires manual pooling.\n\n"+
		"Steps to resolve:\n"+
		"1. Open https://netron.app and upload your model.onnx from: %s\n"+
		"2. Locate the output nodes and identify which one contains the embeddings.\n"+
		"3. Explicitly set this name in your semango.yml using the 'onnx_output_name' property under 'embedding'.",
		availableOutputs, modelOnnxPath)
}

// initONNXSession initializes the ONNX runtime session.
func (le *LocalEmbedder) initONNXSession(modelOnnxPath string, outputName string) (*onnxruntime_go.DynamicAdvancedSession, error) {
	if _, err := os.Stat(modelOnnxPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ONNX model file not found: %s", modelOnnxPath)
	}

	// Get model info to determine input names and output pooling
	inputsInfo, outputsInfo, err := onnxruntime_go.GetInputOutputInfo(modelOnnxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info: %w", err)
	}

	inputNames := make([]string, 0, len(inputsInfo))
	for _, info := range inputsInfo {
		inputNames = append(inputNames, info.Name)
	}
	le.inputNames = inputNames

	// Determine if output is pooled
	for _, info := range outputsInfo {
		if info.Name == outputName {
			if len(info.Dimensions) == 2 {
				le.outputIsPooled = true
			}
			break
		}
	}

	var options *onnxruntime_go.SessionOptions
	if le.enableGPU {
		var err error
		options, err = onnxruntime_go.NewSessionOptions()
		if err != nil {
			slog.Warn("Failed to create ONNX session options for GPU, falling back to CPU", "error", err)
		} else {
			// Try enabling CUDA
			err = options.AppendExecutionProviderCUDA()
			if err != nil {
				slog.Warn("GPU requested but CUDA execution provider failed to initialize, falling back to CPU", "error", err)
				options.Destroy()
				options = nil
			} else {
				slog.Info("GPU acceleration (CUDA) enabled for ONNX session")
			}
		}
	}

	// Create dynamic session
	session, err := onnxruntime_go.NewDynamicAdvancedSession(
		modelOnnxPath,
		inputNames,
		[]string{outputName},
		options,
	)
	if err != nil {
		if options != nil {
			options.Destroy()
		}
		return nil, fmt.Errorf("failed to create dynamic ONNX session: %w", err)
	}

	return session, nil
}

// Embed implements the Embedder interface.
func (le *LocalEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	logger := util.FromContext(ctx)

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	logger.Debug("Starting local embedding", "num_texts", len(texts), "model_path", le.modelOnnxPath)

	// Process texts in batches
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += le.batchSize {
		end := i + le.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := le.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch embedding failed: %w", err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	logger.Debug("Local embedding completed", "num_texts", len(texts), "num_results", len(allEmbeddings))
	return allEmbeddings, nil
}

// embedBatch processes a batch of texts.
func (le *LocalEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Tokenize texts
	inputIDs, attentionMasks, err := le.tokenizeTexts(texts)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Run ONNX inference
	outputs, err := le.runInference(inputIDs, attentionMasks)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Apply pooling
	embeddings, err := le.applyPooling(outputs, attentionMasks)
	if err != nil {
		return nil, fmt.Errorf("pooling failed: %w", err)
	}

	// Normalize embeddings
	for i := range embeddings {
		embeddings[i] = le.normalizeVector(embeddings[i])
	}

	return embeddings, nil
}

// tokenizeTexts tokenizes a batch of texts.
func (le *LocalEmbedder) tokenizeTexts(texts []string) ([][]int64, [][]int64, error) {
	inputIDs := make([][]int64, len(texts))
	attentionMasks := make([][]int64, len(texts))

	for i, text := range texts {
		tokens := le.tokenizer.tokenize(text)
		ids := le.tokenizer.convertTokensToIDs(tokens)

		// Add special tokens
		clsID := int64(le.tokenizer.specialTokens[le.tokenizer.clsToken])
		sepID := int64(le.tokenizer.specialTokens[le.tokenizer.sepToken])

		ids64 := make([]int64, len(ids)+2)
		ids64[0] = clsID
		for j, id := range ids {
			ids64[j+1] = int64(id)
		}
		ids64[len(ids)+1] = sepID

		// Truncate or pad to max length
		if len(ids64) > le.maxLength {
			ids64 = ids64[:le.maxLength]
		}

		mask := make([]int64, len(ids64))
		for j := range mask {
			mask[j] = 1
		}

		// Pad to max length
		padID := int64(le.tokenizer.specialTokens[le.tokenizer.padToken])
		for len(ids64) < le.maxLength {
			ids64 = append(ids64, padID)
			mask = append(mask, 0)
		}

		inputIDs[i] = ids64
		attentionMasks[i] = mask
	}

	return inputIDs, attentionMasks, nil
}

// tokenize splits text into tokens.
func (t *Tokenizer) tokenize(text string) []string {
	if t.doLowerCase {
		text = strings.ToLower(text)
	}

	// Simple whitespace and punctuation tokenization
	// In a real implementation, this would use proper subword tokenization (WordPiece/BPE)
	re := regexp.MustCompile(`\w+|[^\w\s]`)
	tokens := re.FindAllString(text, -1)

	var result []string
	for _, token := range tokens {
		if token != "" {
			result = append(result, token)
		}
	}

	return result
}

// convertTokensToIDs converts tokens to their vocabulary IDs.
func (t *Tokenizer) convertTokensToIDs(tokens []string) []int {
	ids := make([]int, len(tokens))
	unkID := t.specialTokens[t.unkToken]

	for i, token := range tokens {
		if id, exists := t.vocab[token]; exists {
			ids[i] = id
		} else {
			ids[i] = unkID
		}
	}

	return ids
}

// runInference runs ONNX model inference.
func (le *LocalEmbedder) runInference(inputIDs, attentionMasks [][]int64) ([][][]float32, error) {
	batchSize := len(inputIDs)
	seqLength := len(inputIDs[0])

	// Create input tensors for each expected input
	inputShape := onnxruntime_go.NewShape(int64(batchSize), int64(seqLength))
	inputValues := make([]onnxruntime_go.Value, 0, len(le.inputNames))

	for _, name := range le.inputNames {
		var tensor *onnxruntime_go.Tensor[int64]
		var err error
		if name == "input_ids" {
			flat := make([]int64, batchSize*seqLength)
			for i := 0; i < batchSize; i++ {
				copy(flat[i*seqLength:], inputIDs[i])
			}
			tensor, err = onnxruntime_go.NewTensor(inputShape, flat)
		} else if name == "attention_mask" {
			flat := make([]int64, batchSize*seqLength)
			for i := 0; i < batchSize; i++ {
				copy(flat[i*seqLength:], attentionMasks[i])
			}
			tensor, err = onnxruntime_go.NewTensor(inputShape, flat)
		} else if name == "token_type_ids" {
			flat := make([]int64, batchSize*seqLength)
			// Default to all 0s
			tensor, err = onnxruntime_go.NewTensor(inputShape, flat)
		} else {
			// Unknown input, just provide 0s of the same shape
			flat := make([]int64, batchSize*seqLength)
			tensor, err = onnxruntime_go.NewTensor(inputShape, flat)
		}
		if err != nil {
			// Cleanup previously created tensors
			for _, v := range inputValues {
				_ = v.Destroy()
			}
			return nil, fmt.Errorf("failed to create input tensor %s: %w", name, err)
		}
		inputValues = append(inputValues, tensor)
	}
	defer func() {
		for _, v := range inputValues {
			if v != nil {
				_ = v.Destroy()
			}
		}
	}()

	// Create output tensor based on output type
	var outputShape onnxruntime_go.Shape
	if le.outputIsPooled {
		outputShape = onnxruntime_go.NewShape(int64(batchSize), int64(le.dimension))
	} else {
		outputShape = onnxruntime_go.NewShape(int64(batchSize), int64(seqLength), int64(le.dimension))
	}

	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer func() { _ = outputTensor.Destroy() }()

	// Run inference
	err = le.session.Run(inputValues, []onnxruntime_go.Value{outputTensor})
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Get output data and reshape
	outputData := outputTensor.GetData()
	result := make([][][]float32, batchSize)

	if le.outputIsPooled {
		for i := 0; i < batchSize; i++ {
			result[i] = make([][]float32, 1)
			result[i][0] = make([]float32, le.dimension)
			copy(result[i][0], outputData[i*le.dimension:(i+1)*le.dimension])
		}
	} else {
		for i := 0; i < batchSize; i++ {
			result[i] = make([][]float32, seqLength)
			for j := 0; j < seqLength; j++ {
				result[i][j] = make([]float32, le.dimension)
				start := (i*seqLength + j) * le.dimension
				copy(result[i][j], outputData[start:start+le.dimension])
			}
		}
	}

	return result, nil
}

// applyPooling applies pooling strategy to token embeddings.
func (le *LocalEmbedder) applyPooling(outputs [][][]float32, attentionMasks [][]int64) ([][]float32, error) {
	batchSize := len(outputs)
	embeddings := make([][]float32, batchSize)

	for i := 0; i < batchSize; i++ {
		// If the output is already pooled, just use it
		if le.outputIsPooled && len(outputs[i]) == 1 {
			embeddings[i] = outputs[i][0]
		} else if le.poolingConfig.PoolingModeCLSToken {
			// Use CLS token (first token)
			embeddings[i] = outputs[i][0]
		} else if le.poolingConfig.PoolingModeMeanTokens {
			// Mean pooling
			embeddings[i] = le.meanPooling(outputs[i], attentionMasks[i])
		} else if le.poolingConfig.PoolingModeMaxTokens {
			// Max pooling
			embeddings[i] = le.maxPooling(outputs[i], attentionMasks[i])
		} else {
			// Default to mean pooling
			embeddings[i] = le.meanPooling(outputs[i], attentionMasks[i])
		}
	}

	return embeddings, nil
}

// meanPooling applies mean pooling to token embeddings.
func (le *LocalEmbedder) meanPooling(tokenEmbeddings [][]float32, attentionMask []int64) []float32 {
	if len(tokenEmbeddings) == 0 {
		return make([]float32, le.dimension)
	}

	hiddenSize := len(tokenEmbeddings[0])
	pooled := make([]float32, hiddenSize)
	validTokens := 0

	for i, embedding := range tokenEmbeddings {
		if attentionMask[i] == 1 {
			for j, val := range embedding {
				pooled[j] += val
			}
			validTokens++
		}
	}

	if validTokens > 0 {
		for i := range pooled {
			pooled[i] /= float32(validTokens)
		}
	}

	return pooled
}

// maxPooling applies max pooling to token embeddings.
func (le *LocalEmbedder) maxPooling(tokenEmbeddings [][]float32, attentionMask []int64) []float32 {
	if len(tokenEmbeddings) == 0 {
		return make([]float32, le.dimension)
	}

	hiddenSize := len(tokenEmbeddings[0])
	pooled := make([]float32, hiddenSize)

	// Initialize with very negative values
	for i := range pooled {
		pooled[i] = float32(math.Inf(-1))
	}

	for i, embedding := range tokenEmbeddings {
		if attentionMask[i] == 1 {
			for j, val := range embedding {
				if val > pooled[j] {
					pooled[j] = val
				}
			}
		}
	}

	// Handle case where no valid tokens
	for i, val := range pooled {
		if math.IsInf(float64(val), -1) {
			pooled[i] = 0
		}
	}

	return pooled
}

// normalizeVector normalizes a vector to unit length.
func (le *LocalEmbedder) normalizeVector(vector []float32) []float32 {
	var norm float32
	for _, val := range vector {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0 {
		return vector
	}

	normalized := make([]float32, len(vector))
	for i, val := range vector {
		normalized[i] = val / norm
	}

	return normalized
}

// Dimension implements the Embedder interface.
func (le *LocalEmbedder) Dimension() int {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.dimension
}

// Close cleans up resources.
func (le *LocalEmbedder) Close() error {
	if le.session != nil {
		_ = le.session.Destroy()
	}
	return nil
}

// GetSupportedModels returns a list of supported ONNX model names from onnx-models organization.
func GetSupportedModels() []string {
	return []string{
		"onnx-models/all-MiniLM-L6-v2-onnx",
		"onnx-models/all-MiniLM-L12-v2-onnx",
		"onnx-models/all-mpnet-base-v2-onnx",
		"onnx-models/all-mpnet-base-v1-onnx",
		"onnx-models/paraphrase-MiniLM-L6-v2-onnx",
		"onnx-models/paraphrase-MiniLM-L12-v2-onnx",
		"onnx-models/paraphrase-mpnet-base-v2-onnx",
		"onnx-models/paraphrase-multilingual-MiniLM-L12-v2-onnx",
		"onnx-models/paraphrase-multilingual-mpnet-base-v2-onnx",
		"onnx-models/multi-qa-MiniLM-L6-cos-v1-onnx",
		"onnx-models/multi-qa-MiniLM-L6-dot-v1-onnx",
		"onnx-models/multi-qa-distilbert-cos-v1-onnx",
		"onnx-models/multi-qa-distilbert-dot-v1-onnx",
		"onnx-models/multi-qa-mpnet-base-cos-v1-onnx",
		"onnx-models/multi-qa-mpnet-base-dot-v1-onnx",
		"onnx-models/all-distilroberta-v1-onnx",
		"onnx-models/all-roberta-large-v1-onnx",
		"onnx-models/distiluse-base-multilingual-cased-v1-onnx",
		"onnx-models/distiluse-base-multilingual-cased-v2-onnx",
		"onnx-models/LaBSE-onnx",
		"onnx-models/msmarco-bert-base-dot-v5-onnx",
		"onnx-models/msmarco-distilbert-dot-v5-onnx",
		"onnx-models/msmarco-distilbert-base-tas-b-onnx",
		"onnx-models/sentence-t5-base-onnx",
		"onnx-models/sentence-t5-large-onnx",
		"onnx-models/gtr-t5-base-onnx",
		"onnx-models/gtr-t5-large-onnx",
		"onnx-models/jina-embeddings-v2-small-en-onnx",
		"onnx-models/jina-colbert-v1-en-onnx",
		"onnx-models/Splade_PP_en_v1-onnx",
		// Shortened names (without onnx-models/ prefix)
		"all-MiniLM-L6-v2-onnx",
		"all-MiniLM-L12-v2-onnx",
		"all-mpnet-base-v2-onnx",
		"paraphrase-MiniLM-L6-v2-onnx",
		"multi-qa-MiniLM-L6-cos-v1-onnx",
		"jina-embeddings-v2-small-en-onnx",
	}
}

// ValidateModelConfig validates the local embedder configuration.
func ValidateModelConfig(config LocalEmbedderConfig) error {
	if config.ModelPath == "" {
		return fmt.Errorf("model_path is required for local embedder")
	}

	// BatchSize and MaxLength can be 0 (will use defaults), but if specified must be positive
	if config.BatchSize < 0 {
		return fmt.Errorf("batch_size must be non-negative")
	}

	if config.MaxLength < 0 {
		return fmt.Errorf("max_length must be non-negative")
	}

	return nil
}

// Ensure LocalEmbedder implements the Embedder interface.
var _ semango.Embedder = (*LocalEmbedder)(nil)
