package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adit-prawira/neko/internal/ffi"
)

func cliSetup(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "neko_test_cli")
	os.MkdirAll(dir, 0755)
	os.Setenv("NEKO_HOME", dir)
	if err := ffi.Init(dir); err != nil {
		t.Fatalf("engine init failed: %v", err)
	}
	return dir
}

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "neko v0.1.0") {
		t.Errorf("expected version output, got %q", output)
	}
}

func TestCreateCommand(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"create", "test_create", "--dim", "384", "--metric", "cosine"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "collection 'test_create' created") {
		t.Errorf("expected create confirmation, got %q", output)
	}

	ffi.Drop("test_create")
}

func TestCreateCommandMissingDim(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	stderr := new(bytes.Buffer)
	cmd.SetErr(stderr)

	// dim is required, running without it should error
	cmd.SetArgs([]string{"create", "test_missing_dim"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing --dim flag")
	}
}

func TestCreateCommandInvalidMetric(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()

	cmd.SetArgs([]string{"create", "test_bad_metric", "--dim", "384", "--metric", "euclidean"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid metric")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid metric") {
		t.Errorf("expected metric error, got: %v", err)
	}
}

func TestListCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_list_cli")
	if err := ffi.Create("test_list_cli", 256, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test_list_cli") {
		t.Errorf("list output missing collection, got: %q", output)
	}
}

func TestDropCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_drop_cli")
	if err := ffi.Create("test_drop_cli", 128, ffi.MetricDot, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"drop", "test_drop_cli"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drop command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "dropped") {
		t.Errorf("expected drop confirmation, got: %q", output)
	}
}

func TestDropCommandNonexistent(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()

	cmd.SetArgs([]string{"drop", "nonexistent_cli"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for dropping nonexistent collection")
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "neko") {
		t.Errorf("help missing command name, got: %q", output)
	}
}

func writeRawF32(path string, values []float32) error {
	data := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	return os.WriteFile(path, data, 0644)
}

func TestInsertCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_insert_tc")
	if err := ffi.Create("cli_test_insert_tc", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	tmpFile := filepath.Join(dir, "vector.f32")
	if err := writeRawF32(tmpFile, []float32{0.5, 0.6, 0.7}); err != nil {
		t.Fatalf("write vector file: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"insert", "cli_test_insert_tc", "--id", "doc1", "--file", tmpFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("insert command failed: %v", err)
	}

	vector, err := ffi.Get("cli_test_insert_tc", "doc1", 3)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if vector[0] != 0.5 || vector[1] != 0.6 || vector[2] != 0.7 {
		t.Errorf("vector mismatch: got [%v %v %v]", vector[0], vector[1], vector[2])
	}
}

func TestInsertCommandMissingFile(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"insert", "some_collection", "--id", "doc1", "--file", "/nonexistent/vector.f32"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestUpsertCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_upsert_tc")
	if err := ffi.Create("cli_test_upsert_tc", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	tmpFile := filepath.Join(dir, "upsert_vector.f32")
	if err := writeRawF32(tmpFile, []float32{0.5, 0.6, 0.7}); err != nil {
		t.Fatalf("write vector file: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"upsert", "cli_test_upsert_tc", "--id", "doc1", "--file", tmpFile})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("upsert command failed: %v", err)
	}

	vector, err := ffi.Get("cli_test_upsert_tc", "doc1", 3)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if vector[0] != 0.5 || vector[1] != 0.6 || vector[2] != 0.7 {
		t.Errorf("vector mismatch: got [%v %v %v]", vector[0], vector[1], vector[2])
	}
}

func TestUpsertCommandMissingFile(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"upsert", "some_collection", "--id", "doc1", "--file", "/nonexistent/upsert_vector.f32"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestGetCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_get_tc")
	if err := ffi.Create("cli_test_get_tc", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := ffi.Insert("cli_test_get_tc", "doc1", []float32{0.1, 0.2, 0.3}, ""); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "cli_test_get_tc", "doc1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "0.1") || !strings.Contains(output, "0.2") || !strings.Contains(output, "0.3") {
		t.Errorf("get output missing vector values: %q", output)
	}
}

func TestGetCommandNonexistentId(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_getnf_tc")
	if err := ffi.Create("cli_test_getnf_tc", 3, ffi.MetricCosine, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"get", "cli_test_getnf_tc", "no_such_doc"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent id")
	}
}

func TestSearchCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("cli_test_search")
	if err := ffi.Create("cli_test_search", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	ffi.Insert("cli_test_search", "far", []float32{10.0, 0.0, 0.0}, "")
	ffi.Insert("cli_test_search", "near", []float32{2.0, 0.0, 0.0}, "")
	ffi.Insert("cli_test_search", "mid", []float32{5.0, 0.0, 0.0}, "")

	queryFile := filepath.Join(dir, "query.f32")
	writeRawF32(queryFile, []float32{1.0, 0.0, 0.0})

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"search", "cli_test_search", "--file", queryFile, "--k", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "near") || !strings.Contains(output, "mid") {
		t.Errorf("search output missing expected IDs: %q", output)
	}
	if strings.Contains(output, "far") {
		t.Errorf("search output contains far (should not be in top-2): %q", output)
	}
}

func TestSearchCommandMissingFile(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"search", "some_collection", "--file", "/nonexistent/query.f32"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDeleteCommand(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_delete_cli")
	if err := ffi.Create("test_delete_cli", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if err := ffi.Insert("test_delete_cli", "doc1", []float32{1.0, 2.0, 3.0}, ""); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"delete", "test_delete_cli", "doc1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "deleted") {
		t.Errorf("expected delete confirmation, got: %q", output)
	}
}

func TestDeleteCommandNonexistentId(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)

	ffi.Drop("test_delete_nf_cli")
	if err := ffi.Create("test_delete_nf_cli", 3, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"delete", "test_delete_nf_cli", "ghost"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for deleting nonexistent id")
	}
}

func TestDeleteCommandNonexistentCollection(t *testing.T) {
	cliSetup(t)

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"delete", "no_such_clowder_delete", "doc1"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for deleting from nonexistent collection")
	}
}

func TestServeCommandRegistered(t *testing.T) {
	root := NewRootCommand()

	found := false
	for _, subcommand := range root.Commands() {
		if subcommand.Use == "serve" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'serve' subcommand to be registered")
	}
}

func TestServeCommandPortDefault(t *testing.T) {
	root := NewRootCommand()

	serveCommand, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("expected serve command to exist: %v", err)
	}

	portFlag := serveCommand.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatal("expected 'port' flag on serve command")
	}
	if portFlag.DefValue != "3434" {
		t.Errorf("expected port default 3434, got %q", portFlag.DefValue)
	}
}

func TestServeCommandDataDirFlag(t *testing.T) {
	root := NewRootCommand()

	serveCommand, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("expected serve command to exist: %v", err)
	}

	dataDirFlag := serveCommand.Flags().Lookup("data-dir")
	if dataDirFlag == nil {
		t.Fatal("expected 'data-dir' flag on serve command")
	}
	if dataDirFlag.DefValue == "" {
		t.Error("expected non-empty default for 'data-dir' flag")
	}
}

func TestStatsCommandRegistered(t *testing.T) {
	root := NewRootCommand()

	found := false
	for _, subcommand := range root.Commands() {
		if subcommand.Use == "stats" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'stats' subcommand to be registered")
	}
}

func TestStatsCommandJSONFlag(t *testing.T) {
	root := NewRootCommand()

	statsCommand, _, err := root.Find([]string{"stats"})
	if err != nil {
		t.Fatalf("expected stats command to exist: %v", err)
	}

	jsonFlag := statsCommand.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("expected 'json' flag on stats command")
	}
	if jsonFlag.Value.Type() != "bool" {
		t.Errorf("expected json flag to be bool, got %q", jsonFlag.Value.Type())
	}
	if jsonFlag.DefValue != "false" {
		t.Errorf("expected json flag default false, got %q", jsonFlag.DefValue)
	}
}

func TestStatsCommandEmpty(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)
	resetEngineState(t)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"stats"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Collection") {
		t.Errorf("expected header in empty stats output, got %q", output)
	}

	jsonCmd := NewRootCommand()
	jsonBuf := new(bytes.Buffer)
	jsonCmd.SetOut(jsonBuf)
	jsonCmd.SetArgs([]string{"stats", "--json"})
	if err := jsonCmd.Execute(); err != nil {
		t.Fatalf("stats --json command failed: %v", err)
	}

	jsonOutput := strings.TrimSpace(jsonBuf.String())
	var parsed struct {
		Collections []map[string]any `json:"collections"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &parsed); err != nil {
		t.Fatalf("stats --json output is not valid JSON: %v\noutput: %s", err, jsonOutput)
	}
	if len(parsed.Collections) != 0 {
		t.Errorf("expected empty collections array, got %d entries", len(parsed.Collections))
	}
}

func resetEngineState(t *testing.T) {
	t.Helper()
	names, err := ffi.List()
	if err != nil {
		t.Fatalf("ffi.List: %v", err)
	}
	for _, name := range names {
		if err := ffi.Drop(name); err != nil {
			t.Fatalf("ffi.Drop(%q): %v", name, err)
		}
	}
}

func TestStatsCommandWithCollections(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)
	resetEngineState(t)

	if err := ffi.Create("stats_test_docs", 384, ffi.MetricCosine, ""); err != nil {
		t.Fatalf("create docs failed: %v", err)
	}
	if err := ffi.Create("stats_test_images", 768, ffi.MetricL2, ""); err != nil {
		t.Fatalf("create images failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"stats"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}

	output := buf.String()
	for _, headerWord := range []string{"Collection", "Dim", "Vectors", "Disk", "Segments"} {
		if !strings.Contains(output, headerWord) {
			t.Errorf("expected header word %q in output, got %q", headerWord, output)
		}
	}
	if !strings.Contains(output, "stats_test_docs") {
		t.Errorf("expected stats_test_docs in output, got %q", output)
	}
	if !strings.Contains(output, "stats_test_images") {
		t.Errorf("expected stats_test_images in output, got %q", output)
	}
	if !strings.Contains(output, "384") {
		t.Errorf("expected dim 384 in output, got %q", output)
	}
	if !strings.Contains(output, "768") {
		t.Errorf("expected dim 768 in output, got %q", output)
	}
}

func TestStatsCommandJSON(t *testing.T) {
	dir := cliSetup(t)
	defer os.RemoveAll(dir)
	resetEngineState(t)

	if err := ffi.Create("stats_test_json", 128, ffi.MetricDot, ""); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.SetArgs([]string{"stats", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stats --json command failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	var parsed struct {
		Collections []map[string]any `json:"collections"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("stats --json output is not valid JSON: %v\noutput: %s", err, output)
	}
	if len(parsed.Collections) != 1 {
		t.Fatalf("expected 1 collection in JSON, got %d", len(parsed.Collections))
	}

	row := parsed.Collections[0]
	if nameValue, ok := row["name"].(string); !ok || nameValue != "stats_test_json" {
		t.Errorf("expected name=%q in JSON row, got: %v", "stats_test_json", row["name"])
	}
	if dimValue, ok := row["dim"].(float64); !ok || dimValue != 128 {
		t.Errorf("expected dim=128 in JSON row, got: %v", row["dim"])
	}
	if metricValue, ok := row["metric"].(string); !ok || metricValue != "dot" {
		t.Errorf("expected metric=%q in JSON row, got: %v", "dot", row["metric"])
	}
	for _, field := range []string{"vector_count", "storage_bytes", "segments"} {
		if _, ok := row[field]; !ok {
			t.Errorf("expected field %q in JSON row, got keys: %v", field, mapKeys(row))
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input    uint64
		expected string
	}{
		{0, "0B"},
		{1, "1B"},
		{1023, "1023B"},
		{1024, "1KB"},
		{1024 * 1024 - 1, "1023KB"},
		{1024 * 1024, "1MB"},
		{1024 * 1024 * 1024 - 1, "1023MB"},
		{1024 * 1024 * 1024, "1GB"},
	}
	for _, testCase := range cases {
		got := formatBytes(testCase.input)
		if got != testCase.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", testCase.input, got, testCase.expected)
		}
	}
}

func TestCountSegments(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "neko_test_count_segments")
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	emptyCollection := filepath.Join(dir, "collections", "empty_collection")
	if err := os.MkdirAll(emptyCollection, 0755); err != nil {
		t.Fatalf("mkdir empty collection: %v", err)
	}

	withSegments := filepath.Join(dir, "collections", "with_segments")
	if err := os.MkdirAll(withSegments, 0755); err != nil {
		t.Fatalf("mkdir with_segments: %v", err)
	}
	for _, fileName := range []string{"data.vec", "data.meta", "data.vix"} {
		if err := os.WriteFile(filepath.Join(withSegments, fileName), []byte("dummy"), 0644); err != nil {
			t.Fatalf("write %s: %v", fileName, err)
		}
	}
	if err := os.WriteFile(filepath.Join(withSegments, "README.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("write README.txt: %v", err)
	}

	if got := countSegments(dir, "empty_collection"); got != 0 {
		t.Errorf("countSegments(empty_collection) = %d, want 0", got)
	}
	if got := countSegments(dir, "with_segments"); got != 3 {
		t.Errorf("countSegments(with_segments) = %d, want 3", got)
	}
	if got := countSegments(dir, "nonexistent_collection"); got != 0 {
		t.Errorf("countSegments(nonexistent_collection) = %d, want 0", got)
	}
}

func mapKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}
