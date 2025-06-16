package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/omneity-labs/semango/internal/util"
	"github.com/omneity-labs/semango/pkg/semango"
	"github.com/yalue/onnxruntime_go"
)

// LocalEmbedder implements the Embedder interface using local ONNX models.
// It supports sentence transformer models from the onnx-models organization on Hugging Face.
type LocalEmbedder struct {
	modelPath     string
	dimension     int
	maxLength     int
	batchSize     int
	tokenizer     *Tokenizer
	session       *onnxruntime_go.AdvancedSession
	poolingConfig *PoolingConfig
	outputName    string // Cached output name for the ONNX model
	mu            sync.RWMutex
}

// LocalEmbedderConfig holds configuration for the local embedder.
type LocalEmbedderConfig struct {
	ModelPath string // Path to the model directory or onnx-models model name
	CacheDir  string // Directory to cache downloaded models
	BatchSize int    // Batch size for inference
	MaxLength int    // Maximum sequence length
	ModelName string // Specific model name (e.g., "all-MiniLM-L6-v2-onnx")
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
	}

	// Determine if this is a local path or an onnx-models model name
	var modelDir string
	if isLocalPath(config.ModelPath) {
		modelDir = config.ModelPath
	} else {
		// Download from onnx-models organization
		var err error
		modelDir, err = embedder.downloadONNXModel(config.ModelPath, config.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to download model: %w", err)
		}
	}

	embedder.modelPath = modelDir

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

	// Initialize ONNX session
	session, err := embedder.initONNXSession(modelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX session: %w", err)
	}
	embedder.session = session

	// Detect the correct output name for this ONNX model
	outputName, err := embedder.detectOutputName(modelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to detect output name: %w", err)
	}
	embedder.outputName = outputName

	return embedder, nil
}

// isLocalPath checks if the given path is a local file system path.
func isLocalPath(path string) bool {
	// Check if it's an absolute path
	if filepath.IsAbs(path) {
		return true
	}
	// Check if it starts with ./ or ../
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return true
	}
	// Check if it contains file separators but not forward slashes that could be onnx-models names
	// onnx-models names typically have format "model-name-onnx" without additional path separators
	if strings.Contains(path, string(filepath.Separator)) && filepath.Separator != '/' {
		return true
	}
	// If it contains a forward slash, check if it looks like a file path vs onnx-models name
	if strings.Contains(path, "/") {
		// If it has more than one slash or contains dots, likely a file path
		slashCount := strings.Count(path, "/")
		if slashCount > 1 || strings.Contains(path, ".") {
			return true
		}
		// Single slash without dots could be onnx-models name like "onnx-models/all-MiniLM-L6-v2-onnx"
		return false
	}
	return false
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

	modelDir := filepath.Join(cacheDir, strings.ReplaceAll(fullModelName, "/", "_"))

	// Check if model already exists
	if _, err := os.Stat(modelDir); err == nil {
		return modelDir, nil
	}

	// Create model directory
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create model directory: %w", err)
	}

	// List of files to download for an ONNX sentence transformer model
	files := []string{
		"config.json",
		"tokenizer.json",
		"tokenizer_config.json",
		"vocab.txt",
		"model.onnx",
		"1_Pooling/config.json",
		"special_tokens_map.json",
	}

	baseURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main", fullModelName)

	for _, file := range files {
		url := fmt.Sprintf("%s/%s", baseURL, file)
		localPath := filepath.Join(modelDir, file)

		// Create subdirectories if needed
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create directory for %s: %w", file, err)
		}

		if err := le.downloadFile(url, localPath); err != nil {
			// Some files might not exist, continue with others
			continue
		}
	}

	return modelDir, nil
}

// downloadFile downloads a file from URL to local path.
func (le *LocalEmbedder) downloadFile(url, localPath string) error {
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

	_, err = io.Copy(out, resp.Body)
	return err
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
func (le *LocalEmbedder) detectOutputName(modelDir string) (string, error) {
	outputNames := []string{"pooler_output", "last_hidden_state", "output", "logits", "embeddings", "hidden_states", "token_embeddings"}

	modelPath := modelDir + "/model.onnx"
	fmt.Printf("DEBUG detectOutputName: trying model path: %s\n", modelPath)

	for _, outputName := range outputNames {
		// Try to create a session with this output name and actually run inference to validate
		dynamicSession, err := onnxruntime_go.NewDynamicAdvancedSession(
			modelPath,
			[]string{"input_ids", "attention_mask", "token_type_ids"},
			[]string{outputName},
			nil,
		)
		if err != nil {
			fmt.Printf("DEBUG detectOutputName: FAILED to create session with output name: %s, error: %v\n", outputName, err)
			continue
		}

		// Try to run a small inference to validate the output name actually works
		err = le.testInferenceWithSession(dynamicSession, outputName)
		dynamicSession.Destroy()

		if err == nil {
			fmt.Printf("DEBUG detectOutputName: SUCCESS with output name: %s\n", outputName)
			return outputName, nil
		}
		fmt.Printf("DEBUG detectOutputName: FAILED inference test with output name: %s, error: %v\n", outputName, err)
	}

	return "", fmt.Errorf("could not detect valid output name for ONNX model")
}

// testInferenceWithSession tests if inference works with the given session and output name
func (le *LocalEmbedder) testInferenceWithSession(session *onnxruntime_go.DynamicAdvancedSession, outputName string) error {
	// Create minimal test inputs
	batchSize := 1
	seqLength := 10

	inputShape := onnxruntime_go.NewShape(int64(batchSize), int64(seqLength))

	// Create dummy input data
	flatInputIDs := make([]int64, batchSize*seqLength)
	flatAttentionMasks := make([]int64, batchSize*seqLength)
	flatTokenTypeIDs := make([]int64, batchSize*seqLength)
	for i := 0; i < batchSize*seqLength; i++ {
		flatInputIDs[i] = 101 // [CLS] token ID
		flatAttentionMasks[i] = 1
		flatTokenTypeIDs[i] = 0 // Default token type
	}

	inputIDsTensor, err := onnxruntime_go.NewTensor(inputShape, flatInputIDs)
	if err != nil {
		return fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attentionMasksTensor, err := onnxruntime_go.NewTensor(inputShape, flatAttentionMasks)
	if err != nil {
		return fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer attentionMasksTensor.Destroy()

	tokenTypeIDsTensor, err := onnxruntime_go.NewTensor(inputShape, flatTokenTypeIDs)
	if err != nil {
		return fmt.Errorf("failed to create token_type_ids tensor: %w", err)
	}
	defer tokenTypeIDsTensor.Destroy()

	// Create output tensor based on output type
	var outputTensor *onnxruntime_go.Tensor[float32]
	if outputName == "pooler_output" {
		// pooler_output gives sentence-level embeddings: [batch_size, hidden_size]
		outputShape := onnxruntime_go.NewShape(int64(batchSize), int64(le.dimension))
		outputTensor, err = onnxruntime_go.NewEmptyTensor[float32](outputShape)
	} else {
		// token-level outputs: [batch_size, seq_length, hidden_size]
		outputShape := onnxruntime_go.NewShape(int64(batchSize), int64(seqLength), int64(le.dimension))
		outputTensor, err = onnxruntime_go.NewEmptyTensor[float32](outputShape)
	}
	if err != nil {
		return fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Try to run inference
	err = session.Run(
		[]onnxruntime_go.Value{inputIDsTensor, attentionMasksTensor, tokenTypeIDsTensor},
		[]onnxruntime_go.Value{outputTensor},
	)
	if err != nil {
		return fmt.Errorf("failed to run test inference: %w", err)
	}

	return nil
}

// initONNXSession initializes the ONNX runtime session.
func (le *LocalEmbedder) initONNXSession(modelDir string) (*onnxruntime_go.AdvancedSession, error) {
	modelPath := filepath.Join(modelDir, "model.onnx")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ONNX model file not found: %s", modelPath)
	}

	// Initialize ONNX Runtime environment if not already done
	if !onnxruntime_go.IsInitialized() {
		err := onnxruntime_go.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
		}
	}

	// Create session options
	options, err := onnxruntime_go.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	// Create dummy input and output tensors for session initialization
	// We'll use dynamic session later for actual inference
	inputShape := onnxruntime_go.NewShape(1, int64(le.maxLength))
	outputShape := onnxruntime_go.NewShape(1, int64(le.maxLength), int64(le.dimension))

	inputTensor, err := onnxruntime_go.NewEmptyTensor[int64](inputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Create session
	session, err := onnxruntime_go.NewAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		[]onnxruntime_go.Value{inputTensor, inputTensor}, // Use same tensor for both inputs
		[]onnxruntime_go.Value{outputTensor},
		options,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	return session, nil
}

// Embed implements the Embedder interface.
func (le *LocalEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	logger := util.FromContext(ctx)

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	logger.Debug("Starting local embedding", "num_texts", len(texts), "model_path", le.modelPath)

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

	// Create input tensors
	inputShape := onnxruntime_go.NewShape(int64(batchSize), int64(seqLength))

	// Flatten input arrays
	flatInputIDs := make([]int64, batchSize*seqLength)
	flatAttentionMasks := make([]int64, batchSize*seqLength)
	flatTokenTypeIDs := make([]int64, batchSize*seqLength)

	for i := 0; i < batchSize; i++ {
		for j := 0; j < seqLength; j++ {
			flatInputIDs[i*seqLength+j] = inputIDs[i][j]
			flatAttentionMasks[i*seqLength+j] = attentionMasks[i][j]
			flatTokenTypeIDs[i*seqLength+j] = 0 // Default token type
		}
	}

	inputIDsTensor, err := onnxruntime_go.NewTensor(inputShape, flatInputIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attentionMasksTensor, err := onnxruntime_go.NewTensor(inputShape, flatAttentionMasks)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer attentionMasksTensor.Destroy()

	tokenTypeIDsTensor, err := onnxruntime_go.NewTensor(inputShape, flatTokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create token_type_ids tensor: %w", err)
	}
	defer tokenTypeIDsTensor.Destroy()

	// Create dynamic session using the detected output name
	modelPath := le.modelPath + "/model.onnx"
	fmt.Printf("DEBUG runInference: trying model path: %s with output name: %s\n", modelPath, le.outputName)

	dynamicSession, err := onnxruntime_go.NewDynamicAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{le.outputName},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic session: %w", err)
	}
	defer dynamicSession.Destroy()

	// Create output tensor based on output type
	var outputTensor *onnxruntime_go.Tensor[float32]
	if le.outputName == "pooler_output" {
		// pooler_output gives sentence-level embeddings: [batch_size, hidden_size]
		outputShape := onnxruntime_go.NewShape(int64(batchSize), int64(le.dimension))
		outputTensor, err = onnxruntime_go.NewEmptyTensor[float32](outputShape)
	} else {
		// token-level outputs: [batch_size, seq_length, hidden_size]
		outputShape := onnxruntime_go.NewShape(int64(batchSize), int64(seqLength), int64(le.dimension))
		outputTensor, err = onnxruntime_go.NewEmptyTensor[float32](outputShape)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Run inference
	err = dynamicSession.Run(
		[]onnxruntime_go.Value{inputIDsTensor, attentionMasksTensor, tokenTypeIDsTensor},
		[]onnxruntime_go.Value{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run inference: %w", err)
	}

	// Get output data
	outputData := outputTensor.GetData()

	if le.outputName == "pooler_output" {
		// pooler_output gives sentence-level embeddings directly
		// Reshape to [batch_size, 1, hidden_size] for compatibility with pooling code
		result := make([][][]float32, batchSize)
		for i := 0; i < batchSize; i++ {
			result[i] = make([][]float32, 1) // Single "token" representing the sentence
			result[i][0] = make([]float32, le.dimension)
			for k := 0; k < le.dimension; k++ {
				result[i][0][k] = outputData[i*le.dimension+k]
			}
		}
		return result, nil
	} else {
		// Reshape output data back to [batch_size, seq_length, hidden_size]
		result := make([][][]float32, batchSize)
		for i := 0; i < batchSize; i++ {
			result[i] = make([][]float32, seqLength)
			for j := 0; j < seqLength; j++ {
				result[i][j] = make([]float32, le.dimension)
				for k := 0; k < le.dimension; k++ {
					idx := i*seqLength*le.dimension + j*le.dimension + k
					result[i][j][k] = outputData[idx]
				}
			}
		}
		return result, nil
	}
}

// applyPooling applies pooling strategy to token embeddings.
func (le *LocalEmbedder) applyPooling(outputs [][][]float32, attentionMasks [][]int64) ([][]float32, error) {
	batchSize := len(outputs)
	embeddings := make([][]float32, batchSize)

	for i := 0; i < batchSize; i++ {
		// If we used pooler_output, we already have sentence-level embeddings
		if le.outputName == "pooler_output" && len(outputs[i]) == 1 {
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
		le.session.Destroy()
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

	// If it's an onnx-models model name, validate it's supported
	if !isLocalPath(config.ModelPath) {
		supported := GetSupportedModels()
		found := false
		for _, model := range supported {
			if model == config.ModelPath {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unsupported model: %s. Supported models: %v", config.ModelPath, supported)
		}
	}

	return nil
}

// Ensure LocalEmbedder implements the Embedder interface.
var _ semango.Embedder = (*LocalEmbedder)(nil)
