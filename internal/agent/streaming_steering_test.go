package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAGYAdapter_LiveStreamingAndSteering(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock_agent.sh")
	scriptContent := `#!/bin/sh
echo 'Line 1: agent started'
echo 'Line 2: working on task'
# Read interactive steering prompt from stdin
read steer_prompt
echo "Line 3: received steering: $steer_prompt"
echo '{"status": "SUCCESS", "response": "task completed with steering"}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write mock script: %v", err)
	}

	adapter := &AGYAdapter{binaryPath: scriptPath}

	var mu sync.Mutex
	var streamedLines []string
	steerChan := make(chan string, 1)

	req := ExecutionRequest{
		TaskID:       "task-test-stream",
		Prompt:       "test live streaming",
		OutputFormat: "json",
		OnLog: func(stream, line string) {
			mu.Lock()
			streamedLines = append(streamedLines, stream+":"+line)
			mu.Unlock()
		},
		SteerChan: steerChan,
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		steerChan <- "focus on feature X"
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Fatalf("expected status SUCCESS, got: %s", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()

	combined := strings.Join(streamedLines, "\n")
	if !strings.Contains(combined, "Line 1: agent started") {
		t.Errorf("expected Line 1 in streamed lines, got: %s", combined)
	}
	if !strings.Contains(combined, "received steering: focus on feature X") {
		t.Errorf("expected steering prompt in streamed lines, got: %s", combined)
	}
	if result.StructuredOutput == nil || result.StructuredOutput.Response != "task completed with steering" {
		t.Errorf("unexpected structured output: %+v", result.StructuredOutput)
	}
}

func TestAGYAdapter_RegisterSteerFunction(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock_agent_register.sh")
	scriptContent := `#!/bin/sh
echo 'waiting for input'
read val
echo "got: $val"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	adapter := &AGYAdapter{binaryPath: scriptPath}

	var capturedSteer func(prompt string) error
	var mu sync.Mutex

	req := ExecutionRequest{
		TaskID:       "task-test-register",
		Prompt:       "test register steer",
		OutputFormat: "text",
		RegisterSteer: func(steerFn func(prompt string) error) {
			mu.Lock()
			capturedSteer = steerFn
			mu.Unlock()
		},
	}

	go func() {
		for i := 0; i < 50; i++ {
			mu.Lock()
			fn := capturedSteer
			mu.Unlock()
			if fn != nil {
				_ = fn("hello via direct handle")
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result.Stdout, "got: hello via direct handle") {
		t.Fatalf("expected stdout to contain steered text, got: %s", result.Stdout)
	}
}

func TestAGYAdapter_StreamJSONChunksAndResult(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock_stream_agent.sh")
	scriptContent := `#!/bin/sh
echo '{"event":"init","init":{"tools":["run_command"]}}'
echo '{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}'
echo '{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"agent_response","text_delta":"Processing task\n"}}'
echo '{"event":"result","result":{"status":"SUCCESS","response":"Done streaming chunks","duration_seconds":1.2,"num_turns":1}}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	adapter := &AGYAdapter{binaryPath: scriptPath}

	var mu sync.Mutex
	var streamedChunks []string

	req := ExecutionRequest{
		TaskID:       "task-test-stream-json",
		Prompt:       "test streaming json chunks",
		OutputFormat: "stream-json",
		OnLog: func(stream, line string) {
			mu.Lock()
			streamedChunks = append(streamedChunks, line)
			mu.Unlock()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := adapter.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Fatalf("expected status SUCCESS, got: %s", result.Status)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(streamedChunks) != 4 {
		t.Fatalf("expected 4 streamed chunks, got: %d", len(streamedChunks))
	}

	if result.StructuredOutput == nil {
		t.Fatalf("expected StructuredOutput to be parsed from stream-json result")
	}

	if result.StructuredOutput.Response != "Done streaming chunks" {
		t.Errorf("unexpected response: %s", result.StructuredOutput.Response)
	}
	if result.StructuredOutput.Status != "SUCCESS" {
		t.Errorf("unexpected status: %s", result.StructuredOutput.Status)
	}
}
