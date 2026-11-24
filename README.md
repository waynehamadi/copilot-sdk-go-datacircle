# Datacircle tools for the GitHub Copilot SDK (Go)

Find the best GTM APIs. We benchmark all the GTM APIs and we put the results on our website: https://datacircle.dev

This Go module gives a [GitHub Copilot SDK](https://github.com/microsoft/copilot-sdk) session four tools. The tools call Datacircle's open API, https://api.datacircle.dev/v1. You need no key and no account.

| Tool | What the tool gives | Call |
| --- | --- | --- |
| `datacircle_list_apis` | Every GTM API on Datacircle, with the API's slug, number of plans, number of providers and best plan | `GET /apis` |
| `datacircle_best_plans` | One API's plans, sorted by score out of 100, best first. Each plan comes with a rank and a price per 1,000 calls | `GET /apis/{slug}/best` |
| `datacircle_search_plans` | Plans that match a provider or plan name, on all APIs or on one API. Sort by score, provider or price | `GET /plans` |
| `datacircle_provider` | The APIs a provider sells, and the provider's rank on each API | `GET /providers/{slug}` |

## Install

```bash
go get github.com/waynehamadi/copilot-sdk-go-datacircle
```

You need Go 1.26 and the Copilot SDK for Go v2.0.0 or later, `github.com/microsoft/copilot-sdk/go/v2`.

## Add the tools to a session

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	copilot "github.com/microsoft/copilot-sdk/go/v2"
	datacircle "github.com/waynehamadi/copilot-sdk-go-datacircle"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := copilot.NewClient(&copilot.ClientOptions{Mode: copilot.ModeCodingAgent})
	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer client.Stop()

	tools := datacircle.Tools()
	// Add your own tools here too: tools = append(tools, myTool)

	session, err := client.CreateSession(ctx, &copilot.CreateSessionOptions{
		Model:       "gpt-5",
		Tools:       copilot.SessionToolOptions{Definitions: tools},
		Permissions: copilot.SessionPermissionOptions{OnRequest: copilot.PermissionHandler.ApproveAll},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer session.Disconnect()

	reply, err := session.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: "Which email finder API is the best, and what does it cost?",
	})
	if err != nil {
		log.Fatal(err)
	}
	if message, ok := reply.Data.(*copilot.AssistantMessageData); ok {
		fmt.Println(message.Content)
	}
}
```

Before you build, run the SDK's bundler from your `main` package's folder. The bundler embeds the SDK's runtime in your app:

```bash
go get -tool github.com/microsoft/copilot-sdk/go/v2/cmd/bundler
go tool bundler
go build
```

The tools work the same with the SDK's in-process runtime and with the runtime over stdio (`copilot.SpawnedStdioConnection`). The tools also work with your own model and key, through the SDK's custom providers.

If you set `Mode: copilot.ModeEmpty` in `copilot.ClientOptions`, a session runs only the tools listed in `AvailableTools`. Add the Datacircle tools to `AvailableTools`:

```go
tools := copilot.SessionToolOptions{
	Definitions:    datacircle.Tools(),
	AvailableTools: copilot.NewToolSet().AddCustom("*").ToSlice(),
}
```

## Permissions

The four tools read data and change nothing. `SkipPermission` is true by default, so each tool runs with no permission prompt. Set `SkipPermission` to false and your permission handler approves or denies each call:

```go
tools := datacircle.Tools()
for i := range tools {
	tools[i].SkipPermission = false
}
```

## Tool inputs and results

`Tools` makes each tool with the SDK's `copilot.DefineTool`. Each tool has an input struct, such as `BestPlansInput`. The module builds the tool's schema from the input struct. Leave out `limit` and the tool gives 10 results. Each tool returns the answer as JSON.

Each plan comes with a rank, provider, plan, score, price per 1,000 (`perThousand`, in dollars) and a page on datacircle.dev. That page lists every field. The tools take a slug like `email-finder`, an API's name or a provider's name.

The tool gives a failure result when the API gives no data. An unknown slug is one example. The result quotes the API's error message and says what input to send instead. The model reads that result and can call the tool again. The tool gives the same failure result for a network failure or a bad input.

To choose the `HTTPClient` the tools use, call `datacircle.New(cfg).Tools()` with a `datacircle.Config`. `datacircle.New(cfg).Tools()` gives the same tools as `Tools`. Use the config to give the client a timeout or a proxy. Leave `HTTPClient` as `nil` and the tools use `http.DefaultClient`.

```go
tools := datacircle.New(&datacircle.Config{HTTPClient: &http.Client{Timeout: 30 * time.Second}}).Tools()
```

## Call a tool yourself

You can also call each tool as a Go method, without a session:

```go
out, err := datacircle.New(nil).BestPlans(ctx, datacircle.BestPlansInput{Slug: "email-finder", Limit: 3})
if err != nil {
	log.Fatal(err)
}
for _, plan := range out.Plans {
	fmt.Println(plan.Provider, plan.Plan, plan.Page)
}
```

## Network

Each tool call makes one HTTPS GET request to https://api.datacircle.dev/v1 and nothing else. The request holds only the tool's own arguments: no key, no config, nothing from your sessions. The API takes up to 60 calls a minute from one IP address. The tool waits once, for a minute at most, when the API asks the tool to wait. The module depends only on the Copilot SDK and `golang.org/x/text`. The module stores nothing.

## Links

- Datacircle: https://datacircle.dev
- The API: https://datacircle.dev/api
- The API's spec: https://api.datacircle.dev/v1/openapi.json
- What each key means: https://datacircle.dev/fields

MIT license.
