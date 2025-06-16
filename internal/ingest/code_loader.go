package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// "bytes" // Not used yet
	// "io"    // Not used yet

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"

	// Add other language grammars here

	"github.com/omneity-labs/semango/internal/util/log"
	"github.com/omneity-labs/semango/pkg/semango"
)

// CodeLoader handles parsing and extracting content from source code files.
type CodeLoader struct {
	languageMap         map[string]*sitter.Language
	supportedExtensions []string
	// TODO: Config for stripping imports/comments, etc.
}

// Compile-time check to ensure CodeLoader implements semango.Loader
var _ semango.Loader = (*CodeLoader)(nil)

// NewCodeLoader creates a new CodeLoader.
func NewCodeLoader(ctx context.Context /* TODO: add config struct */) (*CodeLoader, error) {
	logger := log.FromContext(ctx)
	lm := make(map[string]*sitter.Language)
	se := make([]string, 0)

	addLang := func(exts []string, lang *sitter.Language, name string) {
		if lang == nil {
			logger.Warn("Tree-sitter language grammar not available or failed to load", "language", name)
			return
		}
		for _, ext := range exts {
			lm[ext] = lang
			se = append(se, ext)
		}
		logger.Debug("Registered Tree-sitter grammar for CodeLoader", "language", name, "extensions", exts)
	}

	addLang([]string{".go"}, golang.GetLanguage(), "Go")
	addLang([]string{".py", ".pyw"}, python.GetLanguage(), "Python")

	if len(lm) == 0 {
		logger.Warn("No Tree-sitter languages were successfully initialized for CodeLoader.")
	}

	return &CodeLoader{
		languageMap:         lm,
		supportedExtensions: se,
	}, nil
}

// Extensions returns the list of file extensions this loader supports.
func (l *CodeLoader) Extensions() []string {
	if l == nil {
		return []string{}
	}
	// Return a copy to prevent modification of the internal slice
	extsCopy := make([]string, len(l.supportedExtensions))
	copy(extsCopy, l.supportedExtensions)
	return extsCopy
}

// Load parses a source code file and extracts its content.
// Actual implementation to be added in subsequent steps.
func (l *CodeLoader) Load(ctx context.Context, path string) ([]semango.Representation, error) {
	logger := log.FromContext(ctx).With("path", path)

	fileContent, err := os.ReadFile(path)
	if err != nil {
		logger.Error("Failed to read file for CodeLoader", "error", err)
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	sitterLang, supported := l.languageMap[ext]
	var languageName string

	// Infer language name for metadata. This is a simple approach.
	// A more robust way would be to store {lang: *sitter.Language, name: string} in NewCodeLoader.
	if supported {
		switch sitterLang { // Compare by pointer, assuming GetLanguage() returns consistent pointers
		case golang.GetLanguage():
			languageName = "Go"
		case python.GetLanguage():
			languageName = "Python"
		// Add cases for other registered languages here
		default:
			// If the language pointer doesn't match a known one, but was in the map,
			// try to use the extension as a fallback name (though this case should be rare if map is built properly).
			logger.Warn("Language found in map but not matched in switch. This is unexpected.", "extension", ext)
			languageName = strings.TrimPrefix(ext, ".")
		}
	} else {
		// If not supported by tree-sitter, languageName will be derived in loadAsPlainText from extension.
		languageName = ""
	}

	if !supported || sitterLang == nil {
		logger.Info("Unsupported file extension or Tree-sitter language not available, falling back to plain text", "extension", ext)
		return l.loadAsPlainText(ctx, path, fileContent, languageName) // Pass inferred or empty languageName
	}

	parser := sitter.NewParser()
	parser.SetLanguage(sitterLang)

	tree, err := parser.ParseCtx(ctx, nil, fileContent)
	if err != nil {
		logger.Error("Tree-sitter parsing failed, falling back to plain text", "error", err, "extension", ext, "language_name", languageName)
		return l.loadAsPlainText(ctx, path, fileContent, languageName)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode == nil || rootNode.HasError() {
		errMsg := "Tree-sitter parsing resulted in error node or nil root"
		if rootNode != nil && rootNode.HasError() {
			errMsg = "Tree-sitter root node has error"
		}
		logger.Warn(errMsg+", falling back to plain text", "extension", ext, "language_name", languageName)
		return l.loadAsPlainText(ctx, path, fileContent, languageName)
	}

	// For now, extract all text content using rootNode.Content().
	// Future enhancements: strip comments, imports, or chunk by functions/classes.
	extractedText := rootNode.Content(fileContent)

	meta := make(map[string]string)
	if languageName != "" {
		meta["language"] = languageName
	} else {
		// Should ideally not happen if supported by Tree-sitter and languageName was inferred
		meta["language"] = strings.TrimPrefix(ext, ".")
	}
	meta["parser"] = "tree-sitter"

	reps := []semango.Representation{{
		Modality: "text",
		Text:     extractedText,
		Meta:     meta,
	}}

	logger.Info("Successfully parsed code file with Tree-sitter", "language", meta["language"], "text_length", len(reps[0].Text))
	return reps, nil
}

// loadAsPlainText is a fallback if Tree-sitter parsing fails or is not applicable.
// This will be filled in later.
func (l *CodeLoader) loadAsPlainText(ctx context.Context, path string, content []byte, detectedLang string) ([]semango.Representation, error) {
	logger := log.FromContext(ctx).With("path", path)
	logger.Info("Loading file as plain text (CodeLoader fallback)", "detected_lang_for_meta", detectedLang)

	meta := make(map[string]string)
	if detectedLang != "" {
		meta["language"] = detectedLang
		meta["parse_mode"] = "plaintext_fallback_from_code"
	} else {
		meta["language"] = strings.TrimPrefix(filepath.Ext(path), ".")
		meta["parse_mode"] = "plaintext_initial_code_loader"
	}

	return []semango.Representation{{
		Modality: "text",
		Text:     string(content),
		Meta:     meta,
	}}, nil
}

// Helper function (example, to be defined if needed for stripping)
// func isCommentNode(node *sitter.Node) bool {
// 	 // This depends heavily on the specific language grammar
// 	 // Example for generic comment types (might not exist in all grammars)
// 	 return node.Type() == "comment" || strings.Contains(node.Type(), "_comment")
// }

// func isImportNode(node *sitter.Node, lang *sitter.Language) bool {
// 	 // This is highly language-specific. Requires checking node types against
// 	 // known import/require/using directive types for each language.
// 	 // e.g., for Go: "import_declaration", for Python: "import_statement", "import_from_statement"
// 	 return false
// }
