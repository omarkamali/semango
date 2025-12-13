package config

// Note: This CUE schema is based on the semango.yml structure from spec.md.
// It defines types and constraints for configuration validation.

#Config: {
	embedding: #EmbeddingConfig
	lexical:   #LexicalConfig
	reranker:  #RerankerConfig
	hybrid:    #HybridConfig
	files:     #FilesConfig
	server:    #ServerConfig
	plugins?:  [...string] // Optional, list of strings
	ui:        #UIConfig
	mcp:       #MCPConfig
	tabular:   #TabularConfig
}

#EmbeddingConfig: {
	provider:         string | *"local" | "openai" | "cohere" | "voyage" // Default: local
	model:            string // Example: text-embedding-3-large
	local_model_path: string | *"models/e5-small.gguf" // Default: models/e5-small.gguf
	batch_size:       int & >=1 & <=512 | *48 // Default: 48
	concurrent:       int & >=1 | *4          // Default: 4
	model_cache_dir:  string // Removed default from here, as it's in semango.yml
}

#LexicalConfig: {
	enabled:    bool | *true
	index_path: string // Removed default from here
	bm25_k1:    float  | *1.2                      // Default: 1.2
	bm25_b:     float  | *0.75                     // Default: 0.75
}

#RerankerConfig: {
	enabled:              bool   | *false                // Default: false
	provider:             string | *"cohere" | "openai" | "local" // Default: cohere
	model:                string | *"rerank-english-v3.0" // Default: rerank-english-v3.0
	batch_size:           int & >=1 | *32                  // Default: 32
	per_request_override: bool   | *true                // Default: true
}

#HybridConfig: {
	vector_weight:  float & >=0.0 & <=1.0 | *0.7 // Default: 0.7
	lexical_weight: float & >=0.0 & <=1.0 | *0.3 // Default: 0.3
	fusion:         string | *"linear" | "rrf"   // Default: linear
}

#FilesConfig: {
	include: [...string] | *["**/*.md", "**/*.go", "**/*.{png,jpg,jpeg}", "**/*.pdf", "**/*.csv", "**/*.json", "**/*.jsonl", "**/*.parquet"]
	exclude: [...string] | *[".git/**", "node_modules/**", "vendor/**"]
	chunk_size: int | *1000
	chunk_overlap: int | *200
}

#ServerConfig: {
	host: string | *"0.0.0.0" // Default: 0.0.0.0
	port: int & >0 & <65536 | *8181 // Default: 8181
	auth: #AuthConfig
	tls_cert?: string // Optional
	tls_key?: string  // Optional, added based on common practice
}

#AuthConfig: {
	type:      string | *"token"          // Default: token
	token_env: string | *"SEMANGO_TOKENS" // Default: SEMANGO_TOKENS
}

#UIConfig: {
	enabled: bool | *true
}

#MCPConfig: {
	enabled: bool | *true
}

#TabularConfig: {
	max_rows_embedded: int & >=1 | *50000
	sampling:          string | *"random" | "stratified"
	min_text_tokens:   int & >=1 | *5
	delimiter?:        string | *","  // CSV delimiter; "\t" for TSV
} 