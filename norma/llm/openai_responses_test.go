package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// TestResponsesStreaming drives the Responses SSE event vocabulary through the
// adapter and asserts the neutral message it accumulates: reasoning summary →
// thinking, output_text → text, function_call added + arg deltas → tool_use.
func TestResponsesStreaming(t *testing.T) {
	body := sse2(
		event("response.created", `{"type":"response.created"}`),
		event("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","delta":"thinking…"}`),
		event("response.output_text.delta", `{"type":"response.output_text.delta","delta":"Hello "}`),
		event("response.output_item.added", `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"Read"}}`),
		event("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","delta":"{\"file_path\":\"a\"}"}`),
		event("response.completed", `{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":11,"output_tokens":6,"input_tokens_details":{"cached_tokens":3}}}}`),
	)
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path=%s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	p, _ := NewProvider(Config{Format: FormatOpenAIResponses, BaseURL: srv.URL, APIKey: "k", Model: "m"})
	acc := NewAccumulator()
	var usage Usage
	for ev, err := range p.Stream(context.Background(), CompletionRequest{Messages: []Message{UserText("go")}}) {
		if err != nil {
			t.Fatalf("stream err: %v", err)
		}
		if ev.Type == SEMessageDelta {
			usage = ev.Usage
		}
		acc.Add(ev)
	}
	// stateless wire: store must be false, and the system prompt is top-level.
	if gotBody["store"] != false {
		t.Fatalf("store=%v, want false", gotBody["store"])
	}
	msg := acc.Message()
	if msg.Text() != "Hello " {
		t.Fatalf("text=%q", msg.Text())
	}
	if u := msg.ToolUses(); len(u) != 1 || u[0].ID != "call_1" || u[0].Name != "Read" || string(u[0].Input) != `{"file_path":"a"}` {
		t.Fatalf("tooluse=%+v", u)
	}
	if usage.InputTokens != 11 || usage.OutputTokens != 6 || usage.CacheReadTokens != 3 {
		t.Fatalf("usage=%+v", usage)
	}
}

// TestResponsesComplete verifies the non-streaming path parses output[] items
// (message / reasoning / function_call) and sends a stateless stream:false body.
func TestResponsesComplete(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path=%s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("content-type", "application/json")
		w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"hmm"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]},{"type":"function_call","call_id":"c9","name":"Bash","arguments":"{\"command\":\"ls\"}"}],"usage":{"input_tokens":10,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}`))
	}))
	defer srv.Close()

	p, _ := NewProvider(Config{Format: FormatOpenAIResponses, BaseURL: srv.URL, APIKey: "k", Model: "m"})
	msg, stop, usage, err := p.Complete(context.Background(), CompletionRequest{Messages: []Message{UserText("go")}})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if gotBody["stream"] != false {
		t.Fatalf("stream=%v, want false", gotBody["stream"])
	}
	if msg.Text() != "Hi" {
		t.Fatalf("text=%q", msg.Text())
	}
	var think string
	for _, b := range msg.Content {
		if b.Type == BlockThinking {
			think = b.Thinking
		}
	}
	if think != "hmm" {
		t.Fatalf("thinking=%q", think)
	}
	if u := msg.ToolUses(); len(u) != 1 || u[0].ID != "c9" || u[0].Name != "Bash" {
		t.Fatalf("tooluse=%+v", u)
	}
	if stop != "tool_use" {
		t.Fatalf("stop=%q", stop)
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 5 || usage.CacheReadTokens != 2 {
		t.Fatalf("usage=%+v", usage)
	}
}

// A reasoning item may carry its thinking in `content` (reasoning_text parts)
// instead of `summary`. The streaming path already handles both event kinds;
// the non-streaming parse must not drop the content variant.
func TestResponsesReasoningTextContent(t *testing.T) {
	msg, _, _, err := parseResponsesResponse([]byte(`{"status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"deep thought"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}]}`))
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	var think string
	for _, b := range msg.Content {
		if b.Type == BlockThinking {
			think = b.Thinking
		}
	}
	if think != "deep thought" {
		t.Fatalf("thinking=%q", think)
	}
	if msg.Text() != "Hi" {
		t.Fatalf("text=%q", msg.Text())
	}
}

// TestToResponsesInput checks the neutral→input-item translation: tool_use →
// function_call, tool_result → function_call_output (call_id paired), text →
// role message, thinking dropped.
func TestToResponsesInput(t *testing.T) {
	msgs := []Message{
		UserText("hi"),
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: BlockThinking, Thinking: "dropped"},
			{Type: BlockText, Text: "calling"},
			{Type: BlockToolUse, ID: "c1", Name: "Bash", Input: []byte(`{"command":"ls"}`)},
		}},
		{Role: RoleUser, Content: []ContentBlock{ToolResultText("c1", "file.txt", false)}},
	}
	in := toResponsesInput(msgs)
	// user message, assistant text message, function_call, function_call_output.
	if len(in) != 4 {
		t.Fatalf("items=%d: %+v", len(in), in)
	}
	if in[0].Role != "user" || in[0].Content != "hi" {
		t.Fatalf("item0=%+v", in[0])
	}
	if in[1].Role != "assistant" || in[1].Content != "calling" {
		t.Fatalf("item1=%+v", in[1])
	}
	if in[2].Type != "function_call" || in[2].CallID != "c1" || in[2].Name != "Bash" {
		t.Fatalf("item2=%+v", in[2])
	}
	if in[3].Type != "function_call_output" || in[3].CallID != "c1" || in[3].Output == nil || *in[3].Output != "file.txt" {
		t.Fatalf("item3=%+v", in[3])
	}
	// thinking must not leak into any item.
	for _, it := range in {
		if strings.Contains(it.Content, "dropped") {
			t.Fatalf("thinking leaked: %+v", it)
		}
	}
}

func TestResponsesBodySanitizesToolExchangeResults(t *testing.T) {
	p := &openaiResponsesProvider{cfg: Config{Model: "fixture"}}
	body, err := p.buildBody(CompletionRequest{Messages: []Message{
		{Role: RoleUser, Content: []ContentBlock{ToolResultText("old", "stale", false)}},
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: BlockToolUse, ID: "a", Name: "Read", Input: []byte(`{}`)},
			{Type: BlockToolUse, ID: "b", Name: "Read", Input: []byte(`{}`)},
		}},
		{Role: RoleUser, Content: []ContentBlock{
			ToolResultText("b", "second", false),
			ToolResultText("a", "first", false),
			ToolResultText("b", "duplicate", false),
			ToolResultText("orphan", "orphan", false),
		}},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Input []orInputItem `json:"input"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	var calls, results []string
	for _, item := range got.Input {
		switch item.Type {
		case "function_call":
			calls = append(calls, item.CallID)
		case "function_call_output":
			if item.Output == nil {
				t.Fatalf("call %q output missing", item.CallID)
			}
			results = append(results, item.CallID+":"+*item.Output)
		}
	}
	if strings.Join(calls, ",") != "a,b" || !slices.Equal(results, []string{"b:second", "a:first"}) {
		t.Fatalf("calls=%v results=%v input=%+v", calls, results, got.Input)
	}
}

func TestResponsesBodyDropsResultsAfterInterveningSummary(t *testing.T) {
	p := &openaiResponsesProvider{cfg: Config{Model: "fixture"}}
	body, err := p.buildBody(CompletionRequest{Messages: []Message{
		{Role: RoleAssistant, Content: []ContentBlock{
			{Type: BlockToolUse, ID: "a", Name: "Read", Input: []byte(`{}`)},
			{Type: BlockToolUse, ID: "b", Name: "Read", Input: []byte(`{}`)},
		}},
		{Role: RoleUser, Content: []ContentBlock{
			TextBlock("summary"),
			ToolResultText("a", "first", false),
			ToolResultText("b", "second", false),
		}},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Input []orInputItem `json:"input"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Input) != 1 || got.Input[0].Role != "user" || got.Input[0].Content != "summary" {
		t.Fatalf("interrupted exchange was not removed: %+v", got.Input)
	}
}

func TestResponsesBodyDoesNotPairAcrossEmptyUserMessage(t *testing.T) {
	p := &openaiResponsesProvider{cfg: Config{Model: "fixture"}}
	body, err := p.buildBody(CompletionRequest{Messages: []Message{
		{Role: RoleAssistant, Content: []ContentBlock{{Type: BlockToolUse, ID: "a", Name: "Read", Input: []byte(`{}`)}}},
		{Role: RoleUser},
		{Role: RoleUser, Content: []ContentBlock{ToolResultText("a", "late", false)}},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Input []orInputItem `json:"input"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Input) != 0 {
		t.Fatalf("tool result crossed an empty user-message boundary: %+v", got.Input)
	}
}

func TestResponsesBodyKeepsEmptyToolResultOutput(t *testing.T) {
	p := &openaiResponsesProvider{cfg: Config{Model: "fixture"}}
	body, err := p.buildBody(CompletionRequest{Messages: []Message{
		{Role: RoleAssistant, Content: []ContentBlock{{Type: BlockToolUse, ID: "empty", Name: "Read", Input: []byte(`{}`)}}},
		{Role: RoleUser, Content: []ContentBlock{ToolResultText("empty", "", false)}},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Input []map[string]json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	for _, item := range got.Input {
		if string(item["type"]) == `"function_call_output"` {
			if output, ok := item["output"]; !ok || string(output) != `""` {
				t.Fatalf("empty output missing or changed: %s", body)
			}
			return
		}
	}
	t.Fatalf("function_call_output missing: %s", body)
}

// event wraps a data payload with a named SSE event line (Responses uses these).
func event(name, data string) string { return "event: " + name + "\ndata: " + data + "\n" }

// sse2 joins named SSE frames into one response body.
func sse2(frames ...string) string { return strings.Join(frames, "\n") + "\n" }
