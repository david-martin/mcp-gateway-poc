# MCP Helper PoC

MCP Helper with Go [Envoy external processor](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_proc_filter).

## Run

```bash
docker-compose up --build
```

A streamable HTTP MCP server will now be available at: http://localhost:8080/

## Test with MCP Inspector

```bash
# Start the inspector
DANGEROUSLY_OMIT_AUTH=true npx @modelcontextprotocol/inspector
```

1. Open http://localhost:6274/?transport=streamable-http&serverUrl=http://localhost:8080#resources
2. Click **Connect**
3. Click **List Tools**

You'll see federated tools from both backend MCP servers (server1 and server2), automatically prefixed to avoid conflicts:

![alt text](mcp-inspector.png)

## Architecture Overview

**Key Components:**

- **MCP Initialize/Tool List**: [`main.go`](main.go) - `handleInitialization()` creates backend sessions, `aggregateTools()` fetches and prefixes tools from servers
- **External Processor**: [`ext-proc/`](ext-proc/) directory handles request/response processing:
  - [`request.go`](ext-proc/request.go) - `extractMCPToolName()` pulls tool name from JSON body, `stripServerPrefix()` removes prefixes, sets `x-mcp-server` routing header, maps session IDs
  - [`response.go`](ext-proc/response.go) - `extractHelperSessionFromBackend()` reverse-maps backend session IDs to helper sessions
- **Routing Rules**: [`envoy.yaml`](envoy.yaml) - routes based on `x-mcp-server` header (`server1`/`server2` → backend clusters, default → helper)

**Flow**: Client → Envoy → Ext-Proc (extracts tool, strips prefix, sets routing headers) → Routes to backend or helper → Response (session reverse mapping) → Client



