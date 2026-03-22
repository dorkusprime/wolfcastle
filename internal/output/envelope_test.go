package output

import (
	"encoding/json"
	"testing"
)

func TestOk_CreatesSuccessResponse(t *testing.T) {
	t.Parallel()
	r := Ok("test-action", map[string]string{"key": "value"})

	if !r.OK {
		t.Error("expected OK=true")
	}
	if r.Action != "test-action" {
		t.Errorf("expected action=%q, got %q", "test-action", r.Action)
	}
	if r.Error != "" {
		t.Error("expected no error message")
	}
	if r.Code != 0 {
		t.Error("expected code=0")
	}
}

func TestErr_CreatesErrorResponse(t *testing.T) {
	t.Parallel()
	r := Err("fail-action", 42, "something went wrong")

	if r.OK {
		t.Error("expected OK=false")
	}
	if r.Action != "fail-action" {
		t.Errorf("expected action=%q, got %q", "fail-action", r.Action)
	}
	if r.Error != "something went wrong" {
		t.Errorf("expected error=%q, got %q", "something went wrong", r.Error)
	}
	if r.Code != 42 {
		t.Errorf("expected code=42, got %d", r.Code)
	}
}

func TestJSON_MarshalingOk(t *testing.T) {
	t.Parallel()
	r := Ok("create", map[string]int{"count": 5})

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["ok"] != true {
		t.Error("expected ok=true in JSON")
	}
	if decoded["action"] != "create" {
		t.Errorf("expected action=create, got %v", decoded["action"])
	}
	// error and code should be omitted for success
	if _, exists := decoded["error"]; exists {
		t.Error("error field should be omitted for success")
	}
	if _, exists := decoded["code"]; exists {
		t.Error("code field should be omitted for success")
	}
	// data should be present
	dataField, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data field as object")
	}
	if dataField["count"] != float64(5) {
		t.Errorf("expected data.count=5, got %v", dataField["count"])
	}
}

func TestPlural_SingularWhenOne(t *testing.T) {
	t.Parallel()
	got := Plural(1, "issue", "issues")
	if got != "1 issue" {
		t.Errorf("expected %q, got %q", "1 issue", got)
	}
}

func TestPlural_PluralWhenZero(t *testing.T) {
	t.Parallel()
	got := Plural(0, "item", "items")
	if got != "0 items" {
		t.Errorf("expected %q, got %q", "0 items", got)
	}
}

func TestPlural_PluralWhenMany(t *testing.T) {
	t.Parallel()
	got := Plural(5, "task", "tasks")
	if got != "5 tasks" {
		t.Errorf("expected %q, got %q", "5 tasks", got)
	}
}

func TestJSON_MarshalingErr(t *testing.T) {
	t.Parallel()
	r := Err("delete", 1, "not found")

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded["ok"] != false {
		t.Error("expected ok=false in JSON")
	}
	if decoded["error"] != "not found" {
		t.Errorf("expected error=%q, got %v", "not found", decoded["error"])
	}
	if decoded["code"] != float64(1) {
		t.Errorf("expected code=1, got %v", decoded["code"])
	}
}
