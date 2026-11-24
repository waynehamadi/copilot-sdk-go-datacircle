package datacircle

import (
	"context"
	"math"
	"strconv"
	"strings"

	copilot "github.com/microsoft/copilot-sdk/go/v2"
)

// Tools gives the four tools for a session's Tools.Definitions, beside the app's own, calling BaseURL with
// http.DefaultClient. A new copy at each call.
func Tools() []copilot.Tool { return New(nil).Tools() }

// Tools gives the four tools for a session's Tools.Definitions, beside the app's own: datacircle_list_apis (GET /apis),
// datacircle_best_plans (GET /apis/{slug}/best), datacircle_search_plans (GET /plans) and datacircle_provider
// (GET /providers/{slug}). Each is made by copilot.DefineTool from its input struct, its schema given each limit's
// bounds and sort's values. Each runs with no permission prompt (SkipPermission), as the four only read: set a tool's
// SkipPermission to false for the session's permission handler to decide. A new copy at each call.
func (t *Toolset) Tools() []copilot.Tool {
	return []copilot.Tool{
		define(ListAPIsName, 200, t.ListAPIs),
		define(BestPlansName, 50, t.BestPlans),
		define(SearchPlansName, 50, t.SearchPlans),
		define(ProviderName, 200, t.Provider),
	}
}

// define makes one of the four tools by copilot.DefineTool. Its handler gives the answer as JSON text, and an answer of
// 4xx, arguments that don't fit its input or a failed network as a failure result whose words the model reads, so that
// it can call again: an error a handler returns reaches the model without its words. The session's stop stops the call.
func define[In, Out any](name string, maxLimit int, run func(context.Context, In) (Out, error)) copilot.Tool {
	tool := copilot.DefineTool(name, abouts[name], func(in In, inv copilot.ToolInvocation) (Out, error) {
		ctx := inv.TraceContext
		if ctx == nil {
			ctx = context.Background()
		}
		return run(ctx, in)
	})
	typed := tool.Handler
	tool.Handler = func(inv copilot.ToolInvocation) (copilot.ToolResult, error) {
		inv.Arguments = loosen(inv.Arguments)
		result, err := typed(inv)
		if err == nil {
			return result, nil
		}
		if inv.TraceContext != nil && inv.TraceContext.Err() != nil {
			return copilot.ToolResult{}, err
		}
		words := err.Error()
		return copilot.ToolResult{TextResultForLLM: words, ResultType: "failure", Error: words}, nil
	}
	tool.SkipPermission = true
	tool.Idempotent = true
	properties, _ := tool.Parameters["properties"].(map[string]any)
	if p, ok := properties["limit"].(map[string]any); ok {
		p["minimum"], p["maximum"], p["default"] = 1, maxLimit, 10
	}
	if p, ok := properties["offset"].(map[string]any); ok {
		p["minimum"], p["default"] = 0, 0
	}
	if p, ok := properties["sort"].(map[string]any); ok {
		p["enum"], p["default"] = []any{"score", "provider", "price"}, "score"
	}
	return tool
}

// loosen reads the model's arguments as the input it meant, where a model that strays (a BYOK model, a small local
// one) can be understood: null is a key left out, and a limit or an offset sent as text is read as a number, a fraction
// cut to its whole part. Anything else is left for the input's decoding to refuse.
func loosen(arguments any) any {
	m, ok := arguments.(map[string]any)
	if !ok {
		if arguments == nil {
			return map[string]any{}
		}
		return arguments
	}
	out := make(map[string]any, len(m))
	for key, value := range m {
		if value == nil {
			continue
		}
		if key == "limit" || key == "offset" {
			if s, ok := value.(string); ok {
				if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
					value = n
				}
			}
			if n, ok := value.(float64); ok && !math.IsInf(n, 0) && !math.IsNaN(n) {
				value = math.Trunc(n)
			}
		}
		out[key] = value
	}
	return out
}
