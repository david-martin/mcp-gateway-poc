package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	var port = flag.String("port", "8081", "Port to listen on")
	flag.Parse()

	log.Println("Starting MCP Test Server 1...")

	hooks := &server.Hooks{}

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		fmt.Printf("beforeAny: %s, %v, %v\n", method, id, message)
	})
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		fmt.Printf("onSuccess: %s, %v, %v, %v\n", method, id, message, result)
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		fmt.Printf("onError: %s, %v, %v, %v\n", method, id, message, err)
	})
	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest) {
		fmt.Printf("beforeInitialize: %v, %v\n", id, message)
	})
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, message any) error {
		fmt.Printf("AddOnRequestInitialization: %v, %v\n", id, message)
		// authorization verification and other preprocessing tasks are performed.
		return nil
	})
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		fmt.Printf("afterInitialize: %v, %v, %v\n", id, message, result)
	})
	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result *mcp.CallToolResult) {
		fmt.Printf("afterCallTool: %v, %v, %v\n", id, message, result)
	})
	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest) {
		fmt.Printf("beforeCallTool: %v, %v\n", id, message)
	})

	// Create MCP server instance with only tool capabilities
	mcpServer := server.NewMCPServer(
		"Test Server 1",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	setupTools(mcpServer)

	// Create streamable HTTP server and start it
	log.Printf("Test Server 1 listening on port %s", *port)
	log.Printf("MCP endpoint: http://localhost:%s", *port)
	streamableServer := server.NewStreamableHTTPServer(mcpServer)

	// // Wrap with logging middleware
	loggingHandler := loggingMiddleware(streamableServer)

	// // Start the HTTP server with the streamable handler
	if err := http.ListenAndServe(":"+*port, loggingHandler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// loggingMiddleware adds comprehensive logging for all HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log all headers for debugging
		log.Printf("=== SERVER1 REQUEST ===")
		log.Printf("Method: %s, URL: %s", r.Method, r.URL.String())
		log.Printf("Headers:")
		for name, values := range r.Header {
			for _, value := range values {
				log.Printf("  %s: %s", name, value)
			}
		}

		// Specifically log session header
		sessionID := r.Header.Get("mcp-session-id")
		if sessionID != "" {
			log.Printf("🔑 [SERVER1] MCP-SESSION-ID: %s", sessionID)
		} else {
			log.Printf("❌ [SERVER1] No mcp-session-id header found")
		}

		// Log request body if present
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("❌ [SERVER1] Error reading request body: %v", err)
			} else if len(bodyBytes) > 0 {
				log.Printf("📝 [SERVER1] Request Body (%d bytes):", len(bodyBytes))
				log.Printf("%s", string(bodyBytes))

				// Restore the body for the actual handler to read
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			} else {
				log.Printf("📝 [SERVER1] Request Body: (empty)")
			}
		}

		log.Printf("=======================")

		// Add HTTP headers to context for tool handlers to access
		ctx := context.WithValue(r.Context(), "http_headers", map[string][]string(r.Header))
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// setupTools configures the two tools for Server 1
func setupTools(s *server.MCPServer) {
	// Echo tool - echoes back the input string
	s.AddTool(mcp.NewTool("echo",
		mcp.WithDescription("Echoes back the input message"),
		mcp.WithString("message",
			mcp.Description("Message to echo back"),
			mcp.Required(),
		),
	), handleEcho)

	// Timestamp tool - returns current time in ISO 8601 format
	s.AddTool(mcp.NewTool("timestamp",
		mcp.WithDescription("Returns the current timestamp in ISO 8601 format"),
	), handleTimestamp)

	// Echo headers tool - returns all headers from the request
	s.AddTool(mcp.NewTool("echo_headers",
		mcp.WithDescription("Returns all headers received by the server"),
	), handleEchoHeaders)

	s.AddTool(mcp.NewTool(
		string("long_running_operation"),
		mcp.WithDescription(
			"Demonstrates a long running operation with progress updates",
		),
		mcp.WithNumber("duration",
			mcp.Description("Duration of the operation in seconds"),
			mcp.DefaultNumber(3),
		),
		mcp.WithNumber("steps",
			mcp.Description("Number of steps in the operation"),
			mcp.DefaultNumber(3),
		),
	), handleLongRunningOperationTool)

	s.AddNotificationHandler("notification", handleNotification)

}

func handleNotification(
	ctx context.Context,
	notification mcp.JSONRPCNotification,
) {
	log.Printf("Received notification: %s", notification.Method)
}

func handleLongRunningOperationTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	progressToken := request.Params.Meta.ProgressToken
	duration, _ := arguments["duration"].(float64)
	steps, _ := arguments["steps"].(float64)
	stepDuration := duration / steps
	server := server.ServerFromContext(ctx)

	log.Printf("long running operation called")
	for i := 1; i < int(steps)+1; i++ {
		time.Sleep(time.Duration(stepDuration * float64(time.Second)))
		if progressToken != nil {
			log.Printf("sending notification")
			err := server.SendNotificationToClient(
				ctx,
				"notifications/progress",
				map[string]any{
					"progress":      i,
					"total":         int(steps),
					"progressToken": progressToken,
					"message":       fmt.Sprintf("Server progress %v%%", int(float64(i)*100/steps)),
				},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to send notification: %w", err)
			}
		}
	}
	log.Printf("long running operation completed")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(
					"Long running operation completed. Duration: %f seconds, Steps: %d.",
					duration,
					int(steps),
				),
			},
		},
	}, nil
}

// handleEcho handles the echo tool
func handleEcho(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("🔧 [SERVER1] handleEcho called")
	message, err := req.RequireString("message")
	if err != nil {
		log.Printf("❌ [SERVER1] Echo error: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'message': %v", err)), nil
	}

	log.Printf("✅ [SERVER1] Echo returning: %s", message)
	return mcp.NewToolResultText(message), nil
}

// handleTimestamp handles the timestamp tool
func handleTimestamp(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("🔧 [SERVER1] handleTimestamp called")
	timestamp := time.Now().Format(time.RFC3339)
	log.Printf("✅ [SERVER1] Timestamp returning: %s", timestamp)
	return mcp.NewToolResultText(timestamp), nil
}

// handleEchoHeaders handles the echo_headers tool
func handleEchoHeaders(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("🔧 [SERVER1] handleEchoHeaders called")

	// Extract HTTP headers from context
	headers := make(map[string]interface{})
	headers["server"] = "server1"
	headers["timestamp"] = time.Now().Format(time.RFC3339)

	// Try to get the HTTP request from context - this depends on the server implementation
	// For now, we'll use a custom context key that we need to set in the middleware
	if httpHeaders, ok := ctx.Value("http_headers").(map[string][]string); ok {
		for name, values := range httpHeaders {
			if len(values) > 0 {
				headers[name] = values[0] // Take first value for simplicity
			}
		}
	} else {
		// If no headers are available, show the context keys for debugging
		headers["context_debug"] = "No HTTP headers found in context"
	}

	result := "Server1 Headers:\n"
	for key, value := range headers {
		result += fmt.Sprintf("  %s: %v\n", key, value)
	}

	log.Printf("✅ [SERVER1] EchoHeaders returning headers")
	return mcp.NewToolResultText(result), nil
}
