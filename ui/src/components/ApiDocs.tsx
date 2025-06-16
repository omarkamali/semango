export function ApiDocs() {
    const jsonExampleRequest = `{
  "query": "string",     // Required: Your search query
  "top_k": "integer",    // Optional: Number of results to return (default: 10, max: 100)
  "filter": "string"   // Optional: Not yet implemented, but reserved for future filtering capabilities
}`;

    const curlExample = `curl -X POST http://localhost:8181/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"your search query", "top_k": 5}'`;

    const jsExample = `fetch('/api/v1/search', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    query: 'your search query',
    top_k: 5,
  }),
})
  .then(response => response.json())
  .then(data => console.log(data))
  .catch(error => console.error('Error:', error));`;

    const jsonResponse = `{
  "results": [
    {
      "rank": 1,
      "score": 0.95,
      "modality": "text",
      "document": {
        "path": "docs/README.md",
        "meta": {
          "source": "github"
        }
      },
      "chunk": "This is a relevant chunk of text from the document.",
      "highlights": {
        "text": [
          { "start": 10, "end": 15 }
        ]
      }
    }
  ],
  "query": "your search query",
  "top_k": 5,
  "took": "12.34ms"
}`;

    const healthResponse = `{
  "status": "healthy",
  "time": "2023-10-27T10:00:00Z"
}`;

    return (
        <div className="max-w-2xl mx-auto text-center py-12">
            <div className="text-4xl font-bold mb-4">📚</div>
            <h2 className="text-2xl font-bold mb-4">API Documentation</h2>
            <p className="text-muted-foreground mb-8">
                Semango offers a simple REST API for programmatic access to its search capabilities.
            </p>

            <div className="text-left space-y-6">
                <h3 className="text-xl font-semibold">Search Endpoint</h3>
                <p className="text-muted-foreground">
                    The primary endpoint for searching your knowledge base.
                </p>

                <div className="bg-card border border-border rounded-lg p-4">
                    <h4 className="font-mono text-sm font-semibold mb-2">POST /api/v1/search</h4>
                    <p className="text-muted-foreground text-sm mb-3">
                        Performs a hybrid search (lexical and semantic) across indexed content.
                    </p>

                    <h5 className="font-medium mb-1">Request Body:</h5>
                    <pre className="bg-muted text-muted-foreground text-xs p-2 rounded overflow-x-auto mb-3">
                        <code className="lang-json" dangerouslySetInnerHTML={{ __html: jsonExampleRequest }} />
                    </pre>

                    <h5 className="font-medium mb-1">Example Request (cURL):</h5>
                    <pre className="bg-muted text-muted-foreground text-xs p-2 rounded overflow-x-auto mb-3">
                        <code className="lang-bash" dangerouslySetInnerHTML={{ __html: curlExample }} />
                    </pre>

                    <h5 className="font-medium mb-1">Example Request (JavaScript/Fetch):</h5>
                    <pre className="bg-muted text-muted-foreground text-xs p-2 rounded overflow-x-auto mb-3">
                        <code className="lang-js" dangerouslySetInnerHTML={{ __html: jsExample }} />
                    </pre>

                    <h5 className="font-medium mb-1">Response Body:</h5>
                    <pre className="bg-muted text-muted-foreground text-xs p-2 rounded overflow-x-auto">
                        <code className="lang-json" dangerouslySetInnerHTML={{ __html: jsonResponse }} />
                    </pre>
                </div>

                <h3 className="text-xl font-semibold">Health Check Endpoint</h3>
                <div className="bg-card border border-border rounded-lg p-4">
                    <h4 className="font-mono text-sm font-semibold mb-2">GET /api/v1/health</h4>
                    <p className="text-muted-foreground text-sm mb-3">
                        Checks the health status of the API server.
                    </p>
                    <h5 className="font-medium mb-1">Example Response:</h5>
                    <pre className="bg-muted text-muted-foreground text-xs p-2 rounded overflow-x-auto">
                        <code className="lang-json" dangerouslySetInnerHTML={{ __html: healthResponse }} />
                    </pre>
                </div>
            </div>
        </div>
    );
} 