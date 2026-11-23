package generate

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

// strict ツールでは、全 object に additionalProperties:false と、全プロパティの required が要る。
func TestProposalSchemaIsStrictCompatible(t *testing.T) {
	props, required := proposalSchema()
	if _, err := json.Marshal(props); err != nil {
		t.Fatal(err)
	}
	// トップレベルの required は Proposal の JSON タグと一致する
	var want []string
	rt := reflect.TypeOf(Proposal{})
	for i := 0; i < rt.NumField(); i++ {
		want = append(want, rt.Field(i).Tag.Get("json"))
	}
	if !slices.Equal(slices.Sorted(slices.Values(required)), slices.Sorted(slices.Values(want))) {
		t.Errorf("required = %v, want %v", required, want)
	}
	for _, name := range want {
		if _, ok := props[name]; !ok {
			t.Errorf("properties に %q が無い", name)
		}
	}

	var check func(path string, node map[string]any)
	check = func(path string, node map[string]any) {
		if node["type"] == "object" {
			if node["additionalProperties"] != false {
				t.Errorf("%s: additionalProperties:false が無い", path)
			}
			p, _ := node["properties"].(map[string]any)
			req, _ := node["required"].([]any)
			if len(p) != len(req) {
				t.Errorf("%s: required(%d) が properties(%d) と一致しない", path, len(req), len(p))
			}
			for k, v := range p {
				check(path+"."+k, v.(map[string]any))
			}
		}
		if items, ok := node["items"].(map[string]any); ok {
			check(path+"[]", items)
		}
	}
	for k, v := range props {
		check(k, v.(map[string]any))
	}

	// pillar の enum は Pillars と同じ
	pillar := props["rationale"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["pillar"].(map[string]any)
	if got := pillar["enum"].([]any); len(got) != len(Pillars) {
		t.Errorf("pillar enum = %v, want %v", got, Pillars)
	}
}
