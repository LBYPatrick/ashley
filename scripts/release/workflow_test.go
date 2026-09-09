package main

import (
	"os"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestReleaseWaitsForAllPlatformsAndGate(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/release.yaml")
	if err != nil {
		t.Fatal(err)
	}
	type job struct {
		Needs       []string
		Strategy    struct{ Matrix map[string][]string }
		Permissions map[string]string
	}
	var workflow struct {
		Jobs struct {
			Package job
			Publish job
		}
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	packageJob, publishJob := workflow.Jobs.Package, workflow.Jobs.Publish
	if !reflect.DeepEqual(packageJob.Needs, []string{"version", "gate"}) || !reflect.DeepEqual(publishJob.Needs, []string{"version", "package"}) {
		t.Fatal("publishing must wait for validation, gate, and every package")
	}
	if !reflect.DeepEqual(packageJob.Strategy.Matrix, map[string][]string{"platform": {"darwin", "linux"}, "arch": {"arm64", "amd64"}}) {
		t.Fatal("incomplete release matrix")
	}
	if !reflect.DeepEqual(publishJob.Permissions, map[string]string{"contents": "write"}) {
		t.Fatal("missing scoped publishing permission")
	}
}
