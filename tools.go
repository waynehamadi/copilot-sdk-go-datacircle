package datacircle

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	ListAPIsName    = "datacircle_list_apis"
	BestPlansName   = "datacircle_best_plans"
	SearchPlansName = "datacircle_search_plans"
	ProviderName    = "datacircle_provider"

	listAPIsAbout    = "Lists the GTM APIs on Datacircle's sheet (https://datacircle.dev), with each API's slug, how many plans and providers it has and its best plan. Call it for an API's slug you don't know: datacircle_best_plans takes a slug, like email-finder. A provider's plans are in datacircle_search_plans and datacircle_provider."
	bestPlansAbout   = "Gives an API's plans best first by their score out of 100 on Datacircle's benchmark (https://datacircle.dev), each with its rank, provider, plan, $ per 1,000 calls and its page: one provider may hold several of the best plans. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	searchPlansAbout = "Searches every plan on Datacircle's sheet (https://datacircle.dev), or one API's, by provider or plan name, sorted by score, provider or price. Gives how many were found and up to limit of them. perThousand is a plan's price of 1,000 items in dollars, pay the plan's whole price in its currency."
	providerAbout    = "Gives the APIs one provider sells and its rank on each on Datacircle's benchmark (https://datacircle.dev), with its best plan there, its best rank first. total counts the APIs it sells, ranked those where it has a rank (its place among the API's providers, 1 the best)."
)

// Names are the four tools' names, in the order Tools gives them.
var Names = []string{ListAPIsName, BestPlansName, SearchPlansName, ProviderName}

// ListAPIsInput is datacircle_list_apis' input.
type ListAPIsInput struct {
	Query string `json:"query,omitempty" jsonschema:"Words of the API's name or slug, like email or linkedin profile: the APIs that hold the most of them come first. The sheet's order when left out"`
	Limit int    `json:"limit,omitempty" jsonschema:"How many APIs to give, 1 to 200. 10 when left out: total says how many matched"`
}

// BestPlansInput is datacircle_best_plans' input.
type BestPlansInput struct {
	Slug  string `json:"slug" jsonschema:"The API's slug, like email-finder or linkedin-profile, as datacircle_list_apis gives it"`
	Limit int    `json:"limit,omitempty" jsonschema:"How many plans to give, 1 to 50. 10 when left out"`
}

// SearchPlansInput is datacircle_search_plans' input.
type SearchPlansInput struct {
	API    string `json:"api,omitempty" jsonschema:"Only this API's plans: its name or slug, like email-finder. A provider's name goes in query, never here. Every API when left out"`
	Query  string `json:"query,omitempty" jsonschema:"Words that must all appear in the provider's or plan's name, like bright data"`
	Sort   string `json:"sort,omitempty" jsonschema:"score (best first, the default), provider (A to Z) or price ($ per 1,000, lowest first)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"How many plans to give, 1 to 50. 10 when left out"`
	Offset int    `json:"offset,omitempty" jsonschema:"How many found plans to skip first. 0 when left out"`
}

// ProviderInput is datacircle_provider's input.
type ProviderInput struct {
	Slug  string `json:"slug" jsonschema:"The provider's slug, like bright-data, or its name, like Bright Data: the name is made a slug"`
	Limit int    `json:"limit,omitempty" jsonschema:"How many of its APIs to give, its best rank first, 1 to 200. 10 when left out"`
}

// Plan is one plan: the keys that choose a plan. Its page on datacircle.dev gives every other.
type Plan struct {
	API                 string   `json:"api" jsonschema:"The API, as the sheet names it"`
	Rank                *int     `json:"rank,omitempty" jsonschema:"The plan's place among its API's scored plans, 1 the best. Absent on a plan with no score"`
	Provider            string   `json:"provider" jsonschema:"Who sells it"`
	Plan                string   `json:"plan" jsonschema:"The plan's name on the provider's pricing page"`
	Score               *float64 `json:"score,omitempty" jsonschema:"Its score out of 100 on Datacircle's benchmark. Absent until its call is benchmarked"`
	PerThousand         *float64 `json:"perThousand,omitempty" jsonschema:"What 1,000 items cost on it, in dollars. Absent with no public price"`
	NotFoundPerThousand *float64 `json:"notFoundPerThousand,omitempty" jsonschema:"What 1,000 items it has no answer for cost, in dollars"`
	Billing             string   `json:"billing" jsonschema:"How you pay, like free, on-demand, one-time, contact or monthly"`
	Pay                 *float64 `json:"pay,omitempty" jsonschema:"The plan's whole price, in its currency"`
	Currency            string   `json:"currency" jsonschema:"The currency of pay, as an ISO 4217 code"`
	Page                string   `json:"page" jsonschema:"The plan's page on datacircle.dev"`
}

// API is one API on the sheet.
type API struct {
	API       string   `json:"api" jsonschema:"The API's name on the sheet"`
	Slug      string   `json:"slug" jsonschema:"Its slug, as datacircle_best_plans takes it"`
	Plans     int      `json:"plans" jsonschema:"How many plans it has"`
	Providers int      `json:"providers" jsonschema:"How many providers sell it"`
	Best      *TopPlan `json:"best,omitempty" jsonschema:"Its best plan"`
	BestPage  string   `json:"bestPage,omitempty" jsonschema:"Its page on datacircle.dev that ranks its providers"`
}

// TopPlan is an API's best plan.
type TopPlan struct {
	Provider    string   `json:"provider"`
	Plan        string   `json:"plan"`
	Score       *float64 `json:"score,omitempty"`
	PerThousand *float64 `json:"perThousand,omitempty"`
}

// ListAPIsOutput is datacircle_list_apis' result.
type ListAPIsOutput struct {
	Total int    `json:"total" jsonschema:"How many APIs matched"`
	APIs  []API  `json:"apis" jsonschema:"Up to limit of them"`
	Note  string `json:"note,omitempty" jsonschema:"Where to look instead, when no API's name holds every word of the query"`
}

// BestPlansOutput is datacircle_best_plans' result.
type BestPlansOutput struct {
	API       string `json:"api"`
	Slug      string `json:"slug"`
	SheetPage string `json:"sheetPage,omitempty" jsonschema:"The API's sheet on datacircle.dev, its plans and their prices"`
	BestPage  string `json:"bestPage,omitempty" jsonschema:"The API's page on datacircle.dev that ranks its providers"`
	Total     int    `json:"total" jsonschema:"How many plans the API has"`
	Plans     []Plan `json:"plans" jsonschema:"Up to limit of its plans, best first"`
}

// SearchPlansOutput is datacircle_search_plans' result.
type SearchPlansOutput struct {
	Total     int    `json:"total" jsonschema:"How many plans were found"`
	Offset    int    `json:"offset" jsonschema:"How many found plans were skipped first"`
	SheetPage string `json:"sheetPage,omitempty" jsonschema:"The API's sheet on datacircle.dev, for one API's plans"`
	BestPage  string `json:"bestPage,omitempty" jsonschema:"The API's page on datacircle.dev that ranks its providers"`
	Plans     []Plan `json:"plans" jsonschema:"Up to limit of the plans found"`
	Note      string `json:"note,omitempty" jsonschema:"What was searched instead of the input, when it found nothing as it was"`
}

// ProviderOutput is datacircle_provider's result.
type ProviderOutput struct {
	About    string        `json:"about" jsonschema:"The counts and its best rank, in words"`
	Provider string        `json:"provider"`
	Slug     string        `json:"slug"`
	Website  string        `json:"website,omitempty"`
	Total    int           `json:"total" jsonschema:"How many APIs it sells"`
	Ranked   int           `json:"ranked" jsonschema:"On how many of them it has a rank"`
	APIs     []ProviderAPI `json:"apis" jsonschema:"Up to limit of them, its best rank first"`
}

// ProviderAPI is one API a provider sells.
type ProviderAPI struct {
	API      string        `json:"api"`
	Slug     string        `json:"slug"`
	Rank     *int          `json:"rank,omitempty" jsonschema:"Its place among the API's providers, 1 the best. Absent where it has none"`
	BestPage string        `json:"bestPage,omitempty"`
	Plans    int           `json:"plans" jsonschema:"How many plans it has on the API"`
	Best     *ProviderPlan `json:"best,omitempty" jsonschema:"Its best plan on the API"`
}

// ProviderPlan is a provider's best plan on one API.
type ProviderPlan struct {
	Plan        string   `json:"plan"`
	Score       *float64 `json:"score,omitempty"`
	PerThousand *float64 `json:"perThousand,omitempty"`
	Page        string   `json:"page"`
}

// Toolset runs the four tools: datacircle_list_apis, datacircle_best_plans, datacircle_search_plans and
// datacircle_provider. Its methods are safe to call from many goroutines.
type Toolset struct {
	c *client
}

// New gives the four tools, calling where cfg says: nil calls BaseURL with http.DefaultClient.
func New(cfg *Config) *Toolset { return &Toolset{c: newClient(cfg)} }

var abouts = map[string]string{ListAPIsName: listAPIsAbout, BestPlansName: bestPlansAbout, SearchPlansName: searchPlansAbout,
	ProviderName: providerAbout}

// advice is a message for the model, like what to send instead: an error it reads.
type advice string

func (a advice) Error() string { return string(a) }

// ListAPIs runs datacircle_list_apis: the APIs on the sheet, those whose name holds the most of the query's words first.
func (t *Toolset) ListAPIs(ctx context.Context, in ListAPIsInput) (ListAPIsOutput, error) {
	var apis []API
	if err := t.c.get(ctx, "/apis", nil, &apis); err != nil {
		return ListAPIsOutput{}, err
	}
	query := strings.TrimSpace(in.Query)
	words := strings.Fields(strings.ReplaceAll(strings.ToLower(query), "-", " "))
	type held struct {
		matched int
		api     API
	}
	var found []held
	for _, api := range apis {
		name := strings.ToLower(api.API + " " + strings.ReplaceAll(api.Slug, "-", " "))
		matched := 0
		for _, w := range words {
			if strings.Contains(name, w) {
				matched++
			}
		}
		if matched > 0 || len(words) == 0 {
			found = append(found, held{matched, api})
		}
	}
	// The most words matched first, the sheet's order within each.
	sort.SliceStable(found, func(i, j int) bool { return found[i].matched > found[j].matched })
	limit := 10
	if in.Limit > 0 {
		limit = clamp(in.Limit, 1, 200)
	}
	out := ListAPIsOutput{Total: len(found), APIs: []API{}}
	for i := 0; i < len(found) && i < limit; i++ {
		out.APIs = append(out.APIs, found[i].api)
	}
	if len(words) > 0 && (len(found) == 0 || found[0].matched < len(words)) {
		// A provider's name sent as an API's, like Bright Data, whose data matches many APIs: where its plans are.
		out.Note = "No API's name holds every word of " + strconv.Quote(query) + ". A provider's plans: datacircle_search_plans with its name as query, or datacircle_provider."
	}
	return out, nil
}

// forProvider tells a model that sent a provider's name to an API's tool where a provider's plans are.
const forProvider = "A provider's plans: datacircle_search_plans with its name as query, like bright data, or datacircle_provider."

// BestPlans runs datacircle_best_plans: an API's plans, best first.
func (t *Toolset) BestPlans(ctx context.Context, in BestPlansInput) (BestPlansOutput, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return BestPlansOutput{}, advice("Give an API's slug, like email-finder: datacircle_list_apis gives every slug. " + forProvider)
	}
	q := url.Values{}
	if in.Limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(in.Limit, 1, 50)))
	}
	var out BestPlansOutput
	if err := t.c.get(ctx, "/apis/"+slug+"/best", q, &out); err != nil {
		if e, ok := err.(*apiError); ok && e.status == 404 {
			// A provider's name sent as the API's slug: the API's words, and where a provider's plans are.
			return BestPlansOutput{}, advice(e.message + ". datacircle_list_apis gives every API's slug. " + forProvider)
		}
		return BestPlansOutput{}, err
	}
	if out.Plans == nil {
		out.Plans = []Plan{}
	}
	return out, nil
}

// SearchPlans runs datacircle_search_plans: the plans whose provider's or plan's name holds the query, on every API or
// one.
func (t *Toolset) SearchPlans(ctx context.Context, in SearchPlansInput) (SearchPlansOutput, error) {
	api, query, note := strings.TrimSpace(in.API), strings.TrimSpace(in.Query), ""
	sortBy, limit, offset := strings.TrimSpace(in.Sort), in.Limit, in.Offset
	out, err := t.search(ctx, api, query, sortBy, limit, offset)
	if e, ok := err.(*apiError); ok && e.status == 400 && strings.Contains(e.message, "api must be") {
		if query != "" {
			// The API's answer names all its APIs: where they are instead.
			return SearchPlansOutput{}, advice("No API is named " + strconv.Quote(api) + ". datacircle_list_apis gives every API's name and slug.")
		}
		// A provider's name sent as the API: the plans whose provider or plan name holds it.
		api, query = "", api
		out, err = t.search(ctx, api, query, sortBy, limit, offset)
		note = "No API is named " + strconv.Quote(query) + ": these are the plans whose provider or plan name holds it."
	}
	if err == nil && out.Total == 0 && strings.Contains(query, "-") {
		// A slug sent as a name, like bright-data, which no name holds: its words, like bright data. A name with a hyphen,
		// like A-Leads, is found as it is first.
		words := strings.Join(strings.Fields(strings.ReplaceAll(query, "-", " ")), " ")
		if again, err := t.search(ctx, api, words, sortBy, limit, offset); err == nil {
			out = again
			note = strings.TrimSpace(note + " No provider's or plan's name holds " + strconv.Quote(query) + ": these are the plans whose name holds " + strconv.Quote(words) + ".")
		}
	}
	if err != nil {
		return SearchPlansOutput{}, err
	}
	out.Note = note
	if out.Plans == nil {
		out.Plans = []Plan{}
	}
	return out, nil
}

func (t *Toolset) search(ctx context.Context, api, query, sort string, limit, offset int) (SearchPlansOutput, error) {
	q := url.Values{}
	if api != "" {
		q.Set("api", api)
	}
	if query != "" {
		q.Set("query", query)
	}
	if sort == "score" || sort == "provider" || sort == "price" {
		q.Set("sort", sort)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(clamp(limit, 1, 50)))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	var out SearchPlansOutput
	err := t.c.get(ctx, "/plans", q, &out)
	return out, err
}

// Provider runs datacircle_provider: the APIs one provider sells, its best rank first, with its best plan on each.
func (t *Toolset) Provider(ctx context.Context, in ProviderInput) (ProviderOutput, error) {
	slug := slugify(in.Slug)
	if slug == "" {
		return ProviderOutput{}, advice("Give a provider's slug or name, like bright-data or Bright Data")
	}
	var v struct {
		Provider, Slug, Website string
		APIs                    []struct {
			API, Slug, BestPage string
			Rank                *int
			Plans               []Plan
		} `json:"apis"`
	}
	if err := t.c.get(ctx, "/providers/"+slug, nil, &v); err != nil {
		if e, ok := err.(*apiError); ok && e.status == 404 {
			// The API's words name its own address: the tool that finds a provider's name instead.
			return ProviderOutput{}, advice(e.message + ". datacircle_search_plans with words of its name as query gives each plan's provider.")
		}
		return ProviderOutput{}, err
	}
	apis := make([]ProviderAPI, 0, len(v.APIs))
	for _, entry := range v.APIs {
		kept := ProviderAPI{API: entry.API, Slug: entry.Slug, Rank: entry.Rank, BestPage: entry.BestPage, Plans: len(entry.Plans)}
		if best := bestPlan(entry.Plans); best != nil {
			kept.Best = &ProviderPlan{Plan: best.Plan, Score: best.Score, PerThousand: best.PerThousand, Page: best.Page}
		}
		apis = append(apis, kept)
	}
	// Its best rank first, the APIs where it has none last, in the API's order.
	sort.SliceStable(apis, func(i, j int) bool {
		return apis[i].Rank != nil && (apis[j].Rank == nil || *apis[i].Rank < *apis[j].Rank)
	})
	ranked := 0
	var bestAt []string
	for _, api := range apis {
		if api.Rank != nil {
			ranked++
			if *api.Rank == *apis[0].Rank {
				bestAt = append(bestAt, api.API)
			}
		}
	}
	limit := 10
	if in.Limit > 0 {
		limit = clamp(in.Limit, 1, 200)
	}
	name := v.Provider
	if name == "" {
		name = slug
	}
	// The counts in words, first of the answer's keys: gpt-oss:120b read total, after 63 APIs, as where it is ranked.
	about := fmt.Sprintf("%s has plans on %d APIs on Datacircle and a rank on %d of them (1 is the best).", name, len(apis), ranked)
	if ranked > 0 {
		about += fmt.Sprintf(" Its best rank: %d, on %s.", *apis[0].Rank, strings.Join(bestAt, ", "))
	}
	out := ProviderOutput{About: about, Provider: v.Provider, Slug: v.Slug, Website: v.Website, Total: len(apis), Ranked: ranked}
	if len(apis) > limit {
		apis = apis[:limit]
	}
	out.APIs = apis
	return out, nil
}

// bestPlan is a provider's best plan on an API: its lowest rank, a plan with no rank having none.
func bestPlan(ps []Plan) *Plan {
	var best *Plan
	for i := range ps {
		if ps[i].Rank != nil && (best == nil || *ps[i].Rank < *best.Rank) {
			best = &ps[i]
		}
	}
	if best == nil && len(ps) > 0 {
		best = &ps[0]
	}
	return best
}
