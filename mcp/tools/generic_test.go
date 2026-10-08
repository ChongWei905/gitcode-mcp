package tools

import "testing"

func TestGenericQueryValuesSupportsPaginationAndRepeatedValues(t *testing.T) {
	values, err := genericQueryValues(map[string]interface{}{
		"page":     float64(2),
		"per_page": float64(100),
		"labels":   []interface{}{"bug", "priority"},
		"draft":    true,
	})
	if err != nil {
		t.Fatalf("genericQueryValues() error = %v", err)
	}
	if values.Get("page") != "2" || values.Get("per_page") != "100" || values.Get("draft") != "true" {
		t.Fatalf("unexpected values: %#v", values)
	}
	if got := values["labels"]; len(got) != 2 || got[0] != "bug" || got[1] != "priority" {
		t.Fatalf("labels = %#v", got)
	}
}

func TestGenericQueryValuesRejectsCredentialFields(t *testing.T) {
	_, err := genericQueryValues(map[string]interface{}{"access_token": "secret"})
	if err == nil {
		t.Fatal("genericQueryValues() accepted access_token")
	}
}
