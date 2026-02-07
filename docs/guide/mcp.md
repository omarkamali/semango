# MCP 🥭

🥭 Semango implements the **Model Context Protocol (MCP)**, allowing LLMs (like Claude) to use Semango as a search tool.

## Usage

Semango provides an MCP server supporting two transport layers:

### 1. Stdio (Local)
For use with local LLM clients like Claude Desktop. The server runs as a child process and communicates via `stdin`/`stdout`.

- **Transport:** JSON-RPC over Standard I/O.
- **Command:** `semango mcp stdio --config /path/to/semango.yml`

**Claude Desktop Configuration:**
To use Semango with Claude Desktop, add this to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "semango": {
      "command": "/usr/local/bin/semango",
      "args": ["mcp", "stdio", "--config", "/absolute/path/to/semango.yml"]
    }
  }
}
```

### 2. SSE (Remote)
For web-based or remote LLM integrations.

- **Transport:** Server-Sent Events (SSE) for downstream, HTTP POST for upstream.
- **Command:** `semango mcp sse --port 8080`

## Tools Provided

The MCP server exposes the following tools:

- `search`: Search across all indexed content using natural language.
  - Arguments: `query` (string), `limit` (number, default: 5).
- `get_stats`: Retrieve indexing progress and document counts.

## Benefits of MCP

By using the MCP server, an LLM can:
1. **Search your codebase** directly to answer questions about logic or dependencies.
2. **Access documentation** indexed by Semango without manual copy-pasting.
3. **Verify statistics** about the search index.

If you need a traditional REST API instead, see the [API Guide](/guide/api).
