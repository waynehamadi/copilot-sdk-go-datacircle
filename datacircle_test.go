package datacircle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	copilot "github.com/microsoft/copilot-sdk/go/v2"
)

// invoke runs one tool's handler as the session runs it, on arguments as the SDK decodes the model's JSON, and gives its
// result: its text, and whether it is a failure.
func invoke(t *testing.T, tools []copilot.Tool, name string, arguments string) (string, bool) {
	t.Helper()
	var args any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range tools {
		if tool.Name == name {
			res, err := tool.Handler(copilot.ToolInvocation{ToolName: name, ToolCallID: "call-1", Arguments: args, TraceContext: context.Background()})
			if err != nil {
				t.Fatalf("%s(%s) returned an error, not a result: %v", name, arguments, err)
			}
			switch res.ResultType {
			case "success":
				if res.Error != "" {
					t.Errorf("%s(%s) succeeded with an error: %+v", name, arguments, res)
				}
			case "failure":
				if res.Error != res.TextResultForLLM || res.Error == "" {
					t.Errorf("%s(%s) failed with no words for the model: %+v", name, arguments, res)
				}
			default:
				t.Errorf("%s(%s) gave a result of type %q", name, arguments, res.ResultType)
			}
			return res.TextResultForLLM, res.ResultType == "failure"
		}
	}
	t.Fatalf("no tool %s", name)
	return "", false
}

func TestDefinitions(t *testing.T) {
	tools := Tools()
	if len(tools) != 4 {
		t.Fatalf("%d tools, want 4", len(tools))
	}
	for i, tool := range tools {
		sent, _ := json.Marshal(tool)
		name := tool.Name
		if name != Names[i] || abouts[name] == "" || tool.Description != abouts[name] || tool.Handler == nil {
			t.Fatalf("tool %d: %s", i, sent)
		}
		if !tool.SkipPermission || !tool.Idempotent || tool.OverridesBuiltInTool || tool.IsTerminal {
			t.Errorf("%s's flags: %s", name, sent)
		}
		switch name {
		case BestPlansName, ProviderName:
			if !strings.Contains(string(sent), `"required":["slug"]`) {
				t.Errorf("%s requires no slug: %s", name, sent)
			}
		default:
			if strings.Contains(string(sent), `"required"`) {
				t.Errorf("%s requires an input: %s", name, sent)
			}
		}
		if strings.Contains(string(sent), `"type":[`) || strings.Contains(string(sent), "null") {
			t.Errorf("%s has a list of types or a null: %s", name, sent)
		}
		if !strings.Contains(string(sent), `"additionalProperties":false`) {
			t.Errorf("%s takes keys of its own: %s", name, sent)
		}
		if name == SearchPlansName && !strings.Contains(string(sent), `"enum":["score","provider","price"]`) {
			t.Errorf("%s's sort has no enum: %s", name, sent)
		}
		var got struct {
			Parameters struct {
				Type       string
				Properties map[string]struct {
					Type, Description string
					Default           any
					Minimum, Maximum  float64
				}
			}
		}
		if json.Unmarshal(sent, &got) != nil || got.Parameters.Type != "object" {
			t.Errorf("%s's definition: %s", name, sent)
		}
		limit := got.Parameters.Properties["limit"]
		high := map[string]float64{ListAPIsName: 200, BestPlansName: 50, SearchPlansName: 50, ProviderName: 200}[name]
		if limit.Type != "integer" || limit.Default != 10.0 || limit.Minimum != 1 || limit.Maximum != high || limit.Description == "" {
			t.Errorf("%s's limit: %+v", name, limit)
		}
	}
	// Each call a new copy: an app that changes one changes none of the next.
	first := Tools()
	first[0].SkipPermission = false
	first[0].Parameters["properties"].(map[string]any)["limit"].(map[string]any)["maximum"] = 3
	if again := Tools(); !again[0].SkipPermission || again[0].Parameters["properties"].(map[string]any)["limit"].(map[string]any)["maximum"] != 200 {
		t.Errorf("a change to one copy reached the next: %+v", again[0])
	}
}

func scripted(t *testing.T) (*httptest.Server, *[]string) {
	var paths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch {
		case r.URL.Path == "/apis":
			w.Write([]byte(`[{"api":"Employee profile","slug":"linkedin-profile","sheetPage":"s","best":null},{"api":"Employee email","slug":"email-finder","plans":3,"best":{"provider":"A","plan":"P","score":70.5,"perThousand":null}}]`))
		case r.URL.Path == "/apis/email-finder/best":
			w.Write([]byte(`{"api":"Employee email","slug":"email-finder","bestPage":null,"total":3,"plans":[{"provider":"A","rank":1,"score":70.5,"perThousand":4.99,"pay":null,"terms":"long"}]}`))
		case r.URL.Path == "/plans" && (r.URL.Query().Get("api") == "Bright Data" || r.URL.Query().Get("api") == "bright-data"):
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"api must be one of: Employee email"}`))
		case r.URL.Path == "/plans" && strings.Contains(r.URL.Query().Get("query"), "-"):
			w.Write([]byte(`{"total":0,"offset":0,"plans":[]}`))
		case r.URL.Path == "/plans":
			w.Write([]byte(`{"total":1,"offset":0,"plans":[{"provider":"Bright Data","rank":null,"score":null}]}`))
		case r.URL.Path == "/providers/bright-data":
			w.Write([]byte(`{"provider":"Bright Data","slug":"bright-data","apis":[{"api":"Employee profile","rank":null,"plans":[{"plan":"C"}]},{"api":"Employee email","rank":4,"plans":[{"plan":"B","rank":9},{"plan":"A","rank":4,"score":61}]}]}`))
		case r.URL.Path == "/providers/down":
			w.WriteHeader(502)
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"no API has the slug nope"}`))
		}
	}))
	t.Cleanup(api.Close)
	return api, &paths
}

func TestCalls(t *testing.T) {
	api, paths := scripted(t)
	tools := New(&Config{BaseURL: api.URL})
	ctx := context.Background()
	run := func(name, arguments string) (string, error) {
		var out any
		var err error
		switch name {
		case ListAPIsName:
			var in ListAPIsInput
			json.Unmarshal([]byte(arguments), &in)
			out, err = tools.ListAPIs(ctx, in)
		case BestPlansName:
			var in BestPlansInput
			json.Unmarshal([]byte(arguments), &in)
			out, err = tools.BestPlans(ctx, in)
		case SearchPlansName:
			var in SearchPlansInput
			json.Unmarshal([]byte(arguments), &in)
			out, err = tools.SearchPlans(ctx, in)
		case ProviderName:
			var in ProviderInput
			json.Unmarshal([]byte(arguments), &in)
			out, err = tools.Provider(ctx, in)
		}
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(out)
		return string(b), nil
	}
	for _, c := range []struct {
		name, arguments string
		fails           bool
		has             string
	}{
		{ListAPIsName, `{"query":"email"}`, false, `"total":1`},
		{ListAPIsName, `{}`, false, `{"total":2,"apis":[{"api":"Employee profile","slug":"linkedin-profile","plans":0,"providers":0},{"api":"Employee email","slug":"email-finder","plans":3,"providers":0,"best":{"provider":"A","plan":"P","score":70.5}}]}`},
		{BestPlansName, `{"slug":"Email Finder","limit":2}`, false, `{"api":"Employee email","slug":"email-finder","total":3,"plans":[{"api":"","rank":1,"provider":"A","plan":"","score":70.5,"perThousand":4.99,"billing":"","currency":"","page":""}]}`},
		{SearchPlansName, `{"api":"Bright Data"}`, false, `No API is named`},
		{SearchPlansName, `{"api":"Bright Data","query":"growth"}`, true, `No API is named "Bright Data". datacircle_list_apis gives every API's name and slug.`},
		{SearchPlansName, `{"api":"bright-data"}`, false, `"plans":[{"api":"","provider":"Bright Data"`},
		{SearchPlansName, `{"api":"bright-data"}`, false, `"note":"No API is named \"bright-data\": these are the plans whose provider or plan name holds it. No provider's or plan's name holds \"bright-data\": these are the plans whose name holds \"bright data\"."`},
		{SearchPlansName, `{"query":"bright-data","sort":"price"}`, false, `"plans":[{"api":"","provider":"Bright Data"`},
		{ProviderName, `{"slug":"Bright Data"}`, false, `"apis":[{"api":"Employee email","slug":"","rank":4,"plans":2,"best":{"plan":"A","score":61,"page":""}}`},
		{ProviderName, `{"slug":"bright-data","limit":1}`, false, `{"about":"Bright Data has plans on 2 APIs on Datacircle and a rank on 1 of them (1 is the best). Its best rank: 4, on Employee email.","provider":"Bright Data","slug":"bright-data","total":2,"ranked":1,"apis":[{"api":"Employee email"`},
		{BestPlansName, `{"slug":"nope"}`, true, `no API has the slug nope. datacircle_list_apis gives every API's slug. A provider's plans: datacircle_search_plans`},
		{BestPlansName, `{}`, true, `Give an API's slug, like email-finder: datacircle_list_apis gives every slug. A provider's plans:`},
		{ListAPIsName, `{"query":"Bright Data"}`, false, `"note":"No API's name holds every word of \"Bright Data\". A provider's plans: datacircle_search_plans`},
		{ProviderName, `{"slug":"down"}`, true, `datacircle: 502 Bad Gateway`},
		{ProviderName, `{"slug":"nope"}`, true, `no API has the slug nope. datacircle_search_plans with words of its name as query gives each plan's provider.`},
		{ProviderName, `{}`, true, `Give a provider's slug or name`},
	} {
		got, err := run(c.name, c.arguments)
		if err != nil {
			got = err.Error()
		}
		if (err != nil) != c.fails || !strings.Contains(got, c.has) {
			t.Errorf("%s(%s) = %s, %v; want it to hold %s", c.name, c.arguments, got, err, c.has)
		}
		if strings.Contains(got, "terms") || strings.Contains(got, "sheetPage") || strings.Contains(got, "null") {
			t.Errorf("%s gave a key it drops or a null: %s", c.name, got)
		}
	}
	if got, _ := run(ListAPIsName, `{"query":"employee email"}`); strings.Contains(got, "note") {
		t.Errorf("datacircle_list_apis notes a query an API's name holds every word of: %s", got)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := tools.ListAPIs(canceled, ListAPIsInput{}); err != context.Canceled {
		t.Errorf("a canceled call gave %v", err)
	}
	if want := "/apis/email-finder/best?limit=2"; (*paths)[2] != want {
		t.Errorf("called %s, want %s", (*paths)[2], want)
	}
	if want := "/plans?query=bright+data&sort=price"; !strings.Contains(strings.Join(*paths, " "), want) {
		t.Errorf("called %v, never %s", *paths, want)
	}

	// Through each tool's own handler, as the session runs it: the answer as JSON text, the API's words as a failure the
	// model reads, arguments read where they can be and refused with words where they can't.
	*paths = nil
	defs := New(&Config{BaseURL: api.URL}).Tools()
	if got, failed := invoke(t, defs, BestPlansName, `{"slug":"email-finder"}`); failed ||
		got != `{"api":"Employee email","slug":"email-finder","total":3,"plans":[{"api":"","rank":1,"provider":"A","plan":"","score":70.5,"perThousand":4.99,"billing":"","currency":"","page":""}]}` {
		t.Errorf("a call gave %s, %v", got, failed)
	}
	if got, failed := invoke(t, defs, BestPlansName, `{"slug":"nope"}`); !failed ||
		got != `no API has the slug nope. datacircle_list_apis gives every API's slug. A provider's plans: datacircle_search_plans with its name as query, like bright data, or datacircle_provider.` {
		t.Errorf("a call of an unknown slug gave %s, %v", got, failed)
	}
	for _, arguments := range []string{"", `null`, `{}`, `{"query":null,"limit":null}`} {
		if got, failed := invoke(t, defs, ListAPIsName, arguments); failed || !strings.Contains(got, `"total":2`) {
			t.Errorf("a call with arguments %q gave %s, %v", arguments, got, failed)
		}
	}
	if got, failed := invoke(t, defs, ProviderName, `{"slug":"down"}`); !failed || got != "datacircle: 502 Bad Gateway" {
		t.Errorf("a call the API failed gave %s, %v", got, failed)
	}
	*paths = nil
	invoke(t, defs, BestPlansName, `{"slug":"email-finder","limit":"2"}`)
	invoke(t, defs, SearchPlansName, `{"query":"growth","limit":2.7,"offset":" 3 ","provider":"Bright Data"}`)
	if want := "/apis/email-finder/best?limit=2 /plans?limit=2&offset=3&query=growth"; strings.Join(*paths, " ") != want {
		t.Errorf("called %v, want %s: a limit sent as text or a fraction read as a whole number, a key of its own dropped", *paths, want)
	}
	for _, bad := range []struct{ name, arguments, has string }{
		{BestPlansName, `{"slug":42}`, "cannot unmarshal number"},
		{SearchPlansName, `{"limit":"many"}`, "cannot unmarshal string"},
		{ListAPIsName, `"email"`, "cannot unmarshal string"},
	} {
		if got, failed := invoke(t, defs, bad.name, bad.arguments); !failed || !strings.Contains(got, bad.has) {
			t.Errorf("%s(%s) gave %s, %v; want a failure holding %s", bad.name, bad.arguments, got, failed, bad.has)
		}
	}
	// The API out of reach: a failure the model reads, the turn going on.
	gone := New(&Config{BaseURL: "http://127.0.0.1:1"}).Tools()
	if got, failed := invoke(t, gone, ListAPIsName, `{}`); !failed || !strings.HasPrefix(got, "datacircle: Get ") {
		t.Errorf("a call with the API out of reach gave %s, %v", got, failed)
	}
	// The session's stop stops the call: an error, no result.
	stopped, stop := context.WithCancel(context.Background())
	stop()
	if _, err := defs[0].Handler(copilot.ToolInvocation{ToolName: ListAPIsName, Arguments: map[string]any{}, TraceContext: stopped}); err == nil {
		t.Error("a call of a stopped session gave a result")
	}
}

// TestLive calls each tool once on the API itself, through its own handler: DATACIRCLE_LIVE=1 go test -run Live.
func TestLive(t *testing.T) {
	if os.Getenv("DATACIRCLE_LIVE") == "" {
		t.Skip("DATACIRCLE_LIVE is unset")
	}
	tools := Tools()
	for _, live := range []struct {
		name, arguments string
		failed          bool
		has             string
	}{
		{ListAPIsName, `{"query":"email finder","limit":3}`, false, `"slug":"email-finder"`},
		{BestPlansName, `{"slug":"email-finder","limit":3}`, false, `"rank":1,`},
		{SearchPlansName, `{"query":"bright data","sort":"price","limit":3}`, false, `"provider":"Bright Data"`},
		{ProviderName, `{"slug":"Bright Data","limit":3}`, false, `"about":"Bright Data has plans on`},
		{BestPlansName, `{"slug":"bright-data"}`, true, `no API has the slug bright-data`},
		{ProviderName, `{"slug":"email-finder"}`, true, `datacircle_search_plans with words of its name`},
	} {
		got, failed := invoke(t, tools, live.name, live.arguments)
		if failed != live.failed || !strings.Contains(got, live.has) {
			t.Errorf("%s(%s) = %s, %v; want it to hold %s", live.name, live.arguments, got, failed, live.has)
		}
		t.Logf("%s(%s) = %.300s", live.name, live.arguments, got)
	}
}
