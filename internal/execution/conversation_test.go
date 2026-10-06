package execution

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LBYPatrick/ashley/internal/history"
)

func TestSessionCallbackLinksExactInvocation(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.Record(history.Invocation{Skill: "raw", AgentType: "codex", CWD: cwd})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Record(history.Invocation{Skill: "raw", AgentType: "codex", CWD: cwd})
	if err != nil {
		t.Fatal(err)
	}
	payload := func(id, dir string) string {
		data, _ := json.Marshal(map[string]string{"session_id": id, "cwd": dir, "source": "startup"})
		return string(data)
	}
	args := []string{path, fmt.Sprint(first), "codex"}
	for _, raw := range []string{`{`, payload("--bad", cwd), payload("session-one", t.TempDir()), strings.Repeat("x", (1<<20)+1)} {
		if err := RecordAgentSession(args, strings.NewReader(raw)); err == nil {
			t.Fatal("accepted invalid callback")
		}
	}
	if err := RecordAgentSession(args, strings.NewReader(payload("session-one", cwd))); err != nil {
		t.Fatal(err)
	}
	if err := RecordAgentSession(args, strings.NewReader(payload("session-one", cwd))); err != nil {
		t.Fatal("not idempotent", err)
	}
	if err := RecordAgentSession(args, strings.NewReader(payload("session-two", cwd))); err == nil {
		t.Fatal("overwrote original conversation")
	}
	args[1] = fmt.Sprint(second)
	args[2] = "kilo"
	if err := RecordAgentSession(args, strings.NewReader(payload("session-two", cwd))); err == nil {
		t.Fatal("accepted wrong agent")
	}
	a, _ := store.Get(first)
	b, _ := store.Get(second)
	if a.AgentSessionID != "session-one" || b.AgentSessionID != "" {
		t.Fatal(a, b)
	}
	if err := RecordAgentSession(nil, strings.NewReader("{}")); err == nil {
		t.Fatal("accepted missing args")
	}
}

func TestTrackingPreservesCommandAndInlineConfig(t *testing.T) {
	for _, agent := range []string{"codex", "opencode", "kilo"} {
		t.Run(agent, func(t *testing.T) {
			t.Setenv("OPENCODE_CONFIG_CONTENT", `{model:"existing",plugin:["user-plugin"],}`)
			t.Setenv("KILO_CONFIG_CONTENT", `{model:"existing",plugin:["user-plugin"],}`)
			cmd := exec.Command("agent", "original prompt")
			job := Job{Agent: agent, HistoryPath: filepath.Join(t.TempDir(), "path with ' quote.db"), InvocationID: 42}
			cleanup, err := trackConversation(cmd, job)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			if cmd.Args[len(cmd.Args)-1] != "original prompt" {
				t.Fatal(cmd.Args)
			}
			if agent == "codex" {
				if !strings.Contains(strings.Join(cmd.Args, " "), "hooks.SessionStart=") || strings.Contains(strings.Join(cmd.Args, " "), "bypass-hook-trust") {
					t.Fatal(cmd.Args)
				}
				return
			}
			key := "OPENCODE_CONFIG_CONTENT="
			if agent == "kilo" {
				key = "KILO_CONFIG_CONTENT="
			}
			var raw string
			for _, env := range cmd.Env {
				if strings.HasPrefix(env, key) {
					raw = strings.TrimPrefix(env, key)
				}
			}
			var cfg struct {
				Model  string
				Plugin []string
			}
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg.Model != "existing" || len(cfg.Plugin) != 2 || cfg.Plugin[0] != "user-plugin" {
				t.Fatal(cfg)
			}
			plugin := strings.TrimPrefix(cfg.Plugin[1], "file://")
			if _, err := os.Stat(plugin); err != nil {
				t.Fatal(err)
			}
			cleanup()
			if _, err := os.Stat(plugin); !os.IsNotExist(err) {
				t.Fatal("plugin not cleaned")
			}
		})
	}
	for _, raw := range []string{"null", `{"plugin":false}`, "{"} {
		if _, err := withTrackingPlugin(raw, "file:///plugin.mjs"); err == nil {
			t.Fatal(raw)
		}
	}
	cmd := exec.Command("agent", "resume", "original")
	cleanup, err := trackConversation(cmd, Job{Agent: "codex", AgentSessionID: "original"})
	cleanup()
	if err != nil || len(cmd.Args) != 3 {
		t.Fatal(cmd.Args, err)
	}
}

func TestTrackingPluginIgnoresChildrenAndOtherDirectories(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required to execute the agent plugin")
	}
	dir := t.TempDir()
	plugin := filepath.Join(dir, "tracking.mjs")
	output := filepath.Join(dir, "events.jsonl")
	callback := []string{node, "-e", `const fs=require('node:fs'); fs.appendFileSync(process.argv[1],fs.readFileSync(0,'utf8')+'\n')`, output}
	if err := os.WriteFile(plugin, []byte(trackingPlugin(callback)), 0600); err != nil {
		t.Fatal(err)
	}
	runner := `const {default:create}=await import(process.argv[1]);
 const hooks=await create({directory:'/workspace'});
 const emit=(info,type='session.created')=>hooks.event({event:{type,properties:{info}}});
 await emit({id:'child',directory:'/workspace',parentID:'root'});
 await emit({id:'other',directory:'/other'});
 await emit({id:'update',directory:'/workspace'},'session.updated');
 await emit({id:'root',directory:'/workspace'});
 await emit({id:'later',directory:'/workspace'});`
	if out, err := exec.Command(node, "--input-type=module", "-e", runner, plugin).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal("expected exactly one event", err, string(data))
	}
	if event.SessionID != "root" || event.CWD != "/workspace" {
		t.Fatal(event)
	}
}
