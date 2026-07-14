package ffi

import (
	"os"
	"path/filepath"
	"testing"
)

func testInit(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "neko_test_go_bridge")
	os.MkdirAll(dir, 0755)
	os.Setenv("NEKO_HOME", dir)
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func testCleanup(name string) {
	_ = Drop(name)
}

func TestDefaultDataDirectory(t *testing.T) {
	t.Run("returns non-empty path", func(t *testing.T) {
		path := DefaultDataDirectory()
		if path == "" {
			t.Error("DefaultDataDirectory returned empty string")
		}
	})

	t.Run("respects NEKO_HOME env var", func(t *testing.T) {
		expected := "/tmp/neko_custom_home"
		os.Setenv("NEKO_HOME", expected)
		defer os.Unsetenv("NEKO_HOME")
		if got := DefaultDataDirectory(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestParseMetric(t *testing.T) {
	t.Run("valid l2 returns 0", func(t *testing.T) {
		code, err := ParseMetric("l2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != MetricL2 {
			t.Errorf("expected %d, got %d", MetricL2, code)
		}
	})

	t.Run("valid cosine returns 1", func(t *testing.T) {
		code, err := ParseMetric("cosine")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != MetricCosine {
			t.Errorf("expected %d, got %d", MetricCosine, code)
		}
	})

	t.Run("valid dot returns 2", func(t *testing.T) {
		code, err := ParseMetric("dot")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != MetricDot {
			t.Errorf("expected %d, got %d", MetricDot, code)
		}
	})

	t.Run("invalid metric returns error", func(t *testing.T) {
		_, err := ParseMetric("euclidean")
		if err == nil {
			t.Error("expected error for invalid metric")
		}
	})
}

func TestVersion(t *testing.T) {
	v := Version()
	if v == "" {
		t.Error("Version returned empty string")
	}
}

func TestInitAndShutdown(t *testing.T) {
	testInit(t)
	if err := ShutDown(); err != nil {
		t.Errorf("ShutDown failed: %v", err)
	}
}

func TestCreateAndDrop(t *testing.T) {
	testInit(t)
	testCleanup("go_test_create")

	if err := Create("go_test_create", 384, MetricCosine, ""); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := Drop("go_test_create"); err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
}

func TestCreateDuplicateFails(t *testing.T) {
	testInit(t)
	testCleanup("go_test_dup")

	if err := Create("go_test_dup", 384, MetricCosine, ""); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := Create("go_test_dup", 384, MetricCosine, ""); err == nil {
		t.Error("expected error for duplicate collection")
	}
	testCleanup("go_test_dup")
}

func TestListAndStats(t *testing.T) {
	testInit(t)
	testCleanup("go_test_list")

	if err := Create("go_test_list", 512, MetricDot, ""); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	names, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, name := range names {
		if name == "go_test_list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("List did not include created collection")
	}

	stats, err := Stats("go_test_list")
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Dim != 512 {
		t.Errorf("expected dim=512, got %d", stats.Dim)
	}
	if stats.Metric != MetricDot {
		t.Errorf("expected metric=%d, got %d", MetricDot, stats.Metric)
	}
	if stats.VectorCount != 0 {
		t.Errorf("expected vector_count=0, got %d", stats.VectorCount)
	}

	testCleanup("go_test_list")
}

func TestDropNonexistentErrors(t *testing.T) {
	testInit(t)
	testCleanup("go_test_nonexistent")

	if err := Drop("go_test_nonexistent"); err == nil {
		t.Error("expected error for nonexistent collection")
	}
}

func TestStatsNonexistentErrors(t *testing.T) {
	testInit(t)
	_, err := Stats("go_test_nonexistent_stats")
	if err == nil {
		t.Error("expected error for nonexistent collection")
	}
}

func TestCreateWithModel(t *testing.T) {
	testInit(t)
	testCleanup("go_test_model")

	if err := Create("go_test_model", 384, MetricCosine, "test-model"); err != nil {
		t.Fatalf("Create with model failed: %v", err)
	}
	testCleanup("go_test_model")
}
