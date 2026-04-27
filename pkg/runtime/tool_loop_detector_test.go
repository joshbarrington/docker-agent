package runtime

import (
	"testing"

	"github.com/docker/docker-agent/pkg/tools"
	"github.com/docker/docker-agent/pkg/tools/builtin"
	bgagent "github.com/docker/docker-agent/pkg/tools/builtin/agent"
)

func TestToolLoopDetector(t *testing.T) {
	makeCalls := func(pairs ...string) []tools.ToolCall {
		var calls []tools.ToolCall
		for i := 0; i < len(pairs); i += 2 {
			calls = append(calls, tools.ToolCall{
				Function: tools.FunctionCall{
					Name:      pairs[i],
					Arguments: pairs[i+1],
				},
			})
		}
		return calls
	}

	tests := []struct {
		name        string
		threshold   int
		exemptTools []string
		batches     [][]tools.ToolCall
		wantTrip    bool // whether any record call returns true
		wantCount   int
	}{
		{
			name:      "no loop with varied calls",
			threshold: 3,
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"b.txt"}`),
				makeCalls("write_file", `{"path":"c.txt"}`),
			},
			wantTrip:  false,
			wantCount: 1,
		},
		{
			name:      "loop detected at exact threshold",
			threshold: 3,
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"a.txt"}`),
			},
			wantTrip:  true,
			wantCount: 3,
		},
		{
			name:      "counter resets when calls change",
			threshold: 3,
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"b.txt"}`), // reset
				makeCalls("read_file", `{"path":"b.txt"}`),
			},
			wantTrip:  false,
			wantCount: 2,
		},
		{
			name:      "empty calls never trigger",
			threshold: 2,
			batches: [][]tools.ToolCall{
				{},
				{},
				{},
			},
			wantTrip:  false,
			wantCount: 0,
		},
		{
			name:      "multi-tool batches compared correctly",
			threshold: 2,
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a"}`, "write_file", `{"path":"b"}`),
				makeCalls("read_file", `{"path":"a"}`, "write_file", `{"path":"b"}`),
			},
			wantTrip:  true,
			wantCount: 2,
		},
		{
			name:      "multi-tool batches differ by one argument",
			threshold: 2,
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a"}`, "write_file", `{"path":"b"}`),
				makeCalls("read_file", `{"path":"a"}`, "write_file", `{"path":"c"}`),
			},
			wantTrip:  false,
			wantCount: 1,
		},
		{
			name:      "reordered JSON keys are treated as identical",
			threshold: 2,
			batches: [][]tools.ToolCall{
				makeCalls("run", `{"cmd":"ls","cwd":"/tmp"}`),
				makeCalls("run", `{"cwd":"/tmp","cmd":"ls"}`),
			},
			wantTrip:  true,
			wantCount: 2,
		},
		{
			name:      "nested JSON key reordering is normalized",
			threshold: 2,
			batches: [][]tools.ToolCall{
				makeCalls("call", `{"a":{"y":2,"x":1},"b":1}`),
				makeCalls("call", `{"b":1,"a":{"x":1,"y":2}}`),
			},
			wantTrip:  true,
			wantCount: 2,
		},
		{
			name:        "exempt background agent polling does not count as a loop",
			threshold:   2,
			exemptTools: []string{bgagent.ToolNameViewBackgroundAgent},
			batches: [][]tools.ToolCall{
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"agent_task_123"}`),
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"agent_task_123"}`),
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"agent_task_123"}`),
			},
			wantTrip:  false,
			wantCount: 0,
		},
		{
			name:        "mixed batch with exempt and non exempt tools still counts",
			threshold:   2,
			exemptTools: []string{bgagent.ToolNameViewBackgroundAgent, builtin.ToolNameViewBackgroundJob},
			batches: [][]tools.ToolCall{
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"agent_task_123"}`, "read_file", `{"path":"a.txt"}`),
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"agent_task_123"}`, "read_file", `{"path":"a.txt"}`),
			},
			wantTrip:  true,
			wantCount: 2,
		},
		{
			name:        "exempt shell background job polling does not count as a loop",
			threshold:   2,
			exemptTools: []string{builtin.ToolNameViewBackgroundJob},
			batches: [][]tools.ToolCall{
				makeCalls(builtin.ToolNameViewBackgroundJob, `{"job_id":"job_1"}`),
				makeCalls(builtin.ToolNameViewBackgroundJob, `{"job_id":"job_1"}`),
			},
			wantTrip:  false,
			wantCount: 0,
		},
		{
			// A looping model cannot evade detection by interleaving a single
			// polling call between identical non-exempt calls. Exempt calls are
			// completely invisible to the detector and do NOT reset the counter.
			name:        "interleaved polling does not evade loop detection",
			threshold:   3,
			exemptTools: []string{bgagent.ToolNameViewBackgroundAgent},
			batches: [][]tools.ToolCall{
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls("read_file", `{"path":"a.txt"}`),
				makeCalls(bgagent.ToolNameViewBackgroundAgent, `{"task_id":"t1"}`), // exempt — counter stays at 2
				makeCalls("read_file", `{"path":"a.txt"}`),                         // consecutive=3 → trips
			},
			wantTrip:  true,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newToolLoopDetector(tt.threshold, tt.exemptTools...)
			var tripped bool
			for _, batch := range tt.batches {
				if d.record(batch) {
					tripped = true
				}
			}
			if tripped != tt.wantTrip {
				t.Errorf("tripped = %v, want %v", tripped, tt.wantTrip)
			}
			if d.consecutive != tt.wantCount {
				t.Errorf("consecutive = %d, want %d", d.consecutive, tt.wantCount)
			}
		})
	}
}

func TestToolLoopDetector_Reset(t *testing.T) {
	calls := []tools.ToolCall{{
		Function: tools.FunctionCall{Name: "read_file", Arguments: `{"path":"a.txt"}`},
	}}

	d := newToolLoopDetector(3)
	d.record(calls)
	d.record(calls)
	if d.consecutive != 2 {
		t.Fatalf("consecutive = %d, want 2 before reset", d.consecutive)
	}
	if d.lastSignature == "" {
		t.Fatalf("lastSignature should be populated before reset")
	}

	d.reset()

	if d.consecutive != 0 {
		t.Errorf("consecutive = %d, want 0 after reset", d.consecutive)
	}
	if d.lastSignature != "" {
		t.Errorf("lastSignature = %q, want empty after reset", d.lastSignature)
	}

	// After reset, identical calls should restart counting from 1, not
	// continue from the pre-reset count.
	if tripped := d.record(calls); tripped {
		t.Errorf("detector tripped on first call after reset; counter not cleared")
	}
	if d.consecutive != 1 {
		t.Errorf("consecutive = %d, want 1 after first record post-reset", d.consecutive)
	}
}
