package config

// This file duplicates docs/config.cue so that it can be embedded into the
// binary without using relative ".." paths which are disallowed by go:embed.
// Keep the contents in sync with docs/config.cue.

#Config: {
	embedding: #EmbeddingConfig
	lexical:   #LexicalConfig
	reranker:   #RerankerConfig
	hybrid:    #HybridConfig
	files:     #FilesConfig
	server:    #ServerConfig
	plugins?:  [...string]
	ui:        #UIConfig
	mcp:       #MCPConfig
	tabular:   #TabularConfig
}

#EmbeddingConfig: {
	provider:         string | *"local" | "openai" | "cohere" | "voyage"
	model:            string
	local_model_path: string | *"models/e5-small.gguf"
	batch_size:       int & >=1 & <=512 | *48
	concurrent:       int & >=1 | *4
	model_cache_dir:  string
}

#LexicalConfig: {
	enabled:    bool | *true
	index_path: string
	bm25_k1:    float  | *1.2
	bm25_b:     float  | *0.75
}

#RerankerConfig: {
	enabled:              bool   | *false
	provider:             string | *"cohere" | "openai" | "local"
	model:                string | *"rerank-english-v3.0"
	batch_size:           int & >=1 | *32
	per_request_override: bool   | *true
}

#HybridConfig: {
	vector_weight:  float & >=0.0 & <=1.0 | *0.7
	lexical_weight: float & >=0.0 & <=1.0 | *0.3
	fusion:         string | *"linear" | "rrf"
}

#FilesConfig: {
	include: [...string] | *["**/*.md", "**/*.go", "**/*.{png,jpg,jpeg}", "**/*.pdf", "**/*.csv", "**/*.json", "**/*.jsonl", "**/*.parquet"]
	exclude: [...string] | *[".git/**", "node_modules/**", "vendor/**"]
	chunk_size: int | *1000
	chunk_overlap: int | *200
}

#ServerConfig: {
	host: string | *"0.0.0.0"
	port: int & >0 & <65536 | *8181
	auth: #AuthConfig
	tls_cert?: string
	tls_key?: string
}

#AuthConfig: {
	type:      string | *"token"
	token_env: string | *"SEMANGO_TOKENS"
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
	delimiter?:        string | *","  // for CSV/TSV; "\t" for TSV
} 