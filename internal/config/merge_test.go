package config

import (
	"testing"
)

func TestDeepMerge_BasicKeyOverride(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"a": "old", "b": "keep"}
	src := map[string]any{"a": "new"}
	got := DeepMerge(dst, src)
	if got["a"] != "new" {
		t.Errorf("expected a=new, got %v", got["a"])
	}
	if got["b"] != "keep" {
		t.Errorf("expected b=keep, got %v", got["b"])
	}
}

func TestDeepMerge_DeepNestedMerge(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"outer": map[string]any{
			"inner1": "v1",
			"inner2": "v2",
		},
	}
	src := map[string]any{
		"outer": map[string]any{
			"inner2": "v2-new",
			"inner3": "v3",
		},
	}
	got := DeepMerge(dst, src)
	outer := got["outer"].(map[string]any)
	if outer["inner1"] != "v1" {
		t.Errorf("inner1 should be preserved, got %v", outer["inner1"])
	}
	if outer["inner2"] != "v2-new" {
		t.Errorf("inner2 should be overridden, got %v", outer["inner2"])
	}
	if outer["inner3"] != "v3" {
		t.Errorf("inner3 should be added, got %v", outer["inner3"])
	}
}

func TestDeepMerge_ArrayFullReplacement(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"items": []any{"a", "b", "c"},
	}
	src := map[string]any{
		"items": []any{"x"},
	}
	got := DeepMerge(dst, src)
	items := got["items"].([]any)
	if len(items) != 1 || items[0] != "x" {
		t.Errorf("array should be fully replaced, got %v", items)
	}
}

func TestDeepMerge_NullDeletion(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"a": "val", "b": "keep"}
	src := map[string]any{"a": nil}
	got := DeepMerge(dst, src)
	if _, exists := got["a"]; exists {
		t.Error("key 'a' should be deleted by null")
	}
	if got["b"] != "keep" {
		t.Errorf("key 'b' should remain, got %v", got["b"])
	}
}

func TestDeepMerge_EmptyMaps(t *testing.T) {
	t.Parallel()

	// dst nil
	got := DeepMerge(nil, map[string]any{"x": 1})
	if got["x"] != float64(1) && got["x"] != 1 {
		t.Errorf("expected x=1, got %v", got["x"])
	}

	// src empty
	dst := map[string]any{"y": 2}
	got2 := DeepMerge(dst, map[string]any{})
	if got2["y"] != 2 {
		t.Errorf("expected y=2, got %v", got2["y"])
	}
}

func TestDeepMerge_NestedNullDeletion(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"outer": map[string]any{
			"keep":   "yes",
			"remove": "gone",
		},
	}
	src := map[string]any{
		"outer": map[string]any{
			"remove": nil,
		},
	}
	got := DeepMerge(dst, src)
	outer := got["outer"].(map[string]any)
	if _, exists := outer["remove"]; exists {
		t.Error("nested key 'remove' should be deleted")
	}
	if outer["keep"] != "yes" {
		t.Errorf("nested key 'keep' should remain, got %v", outer["keep"])
	}
}

func TestDeepMerge_DoesNotMutateDst(t *testing.T) {
	t.Parallel()
	dst := map[string]any{
		"a": "original",
		"nested": map[string]any{
			"x": "keep",
		},
	}
	src := map[string]any{
		"a": "changed",
		"nested": map[string]any{
			"x": "modified",
			"y": "added",
		},
	}
	_ = DeepMerge(dst, src)

	// Verify original dst was not mutated
	if dst["a"] != "original" {
		t.Errorf("dst should not be mutated, 'a' got %v", dst["a"])
	}
	nested := dst["nested"].(map[string]any)
	if nested["x"] != "keep" {
		t.Errorf("dst nested should not be mutated, 'x' got %v", nested["x"])
	}
	if _, exists := nested["y"]; exists {
		t.Error("dst nested should not have key 'y' added")
	}
}

func TestDeepMerge_NoOverlap(t *testing.T) {
	t.Parallel()
	dst := map[string]any{"a": 1}
	src := map[string]any{"b": 2}
	got := DeepMerge(dst, src)
	if got["a"] != 1 {
		t.Errorf("expected a=1, got %v", got["a"])
	}
	if got["b"] != 2 {
		t.Errorf("expected b=2, got %v", got["b"])
	}
}
