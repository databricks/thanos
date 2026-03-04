package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginGoldenOutput(t *testing.T) {
	pluginBin := findPluginBinary(t)
	protocBin := findProtoc(t)

	gogoProtoRoot := modDir(t, "github.com/gogo/protobuf")
	vtprotoInclude := filepath.Join(modDir(t, "github.com/planetscale/vtprotobuf"), "include")
	vtpoolProtoDir := filepath.Join(repoRoot(t), "pkg", "vtproto", "gen")

	testdataDir := filepath.Join(vtpoolProtoDir, "test", "testdata")
	goldenPath := filepath.Join(testdataDir, "test_grpc_vtpool.pb.go.golden")

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}

	outDir := t.TempDir()

	cmd := exec.Command(protocBin,
		"--go-grpc-vtpool_out="+outDir,
		"--go-grpc-vtpool_opt=paths=source_relative",
		"--go-grpc-vtpool_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb",
		"-I="+testdataDir,
		"-I="+filepath.Join(gogoProtoRoot, "protobuf"),
		"-I="+vtprotoInclude,
		"-I="+vtpoolProtoDir,
		filepath.Join(testdataDir, "test.proto"),
	)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(pluginBin)+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	generatedPath := filepath.Join(outDir, "test_grpc_vtpool.pb.go")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	if string(generated) != string(golden) {
		t.Errorf("generated output differs from golden file.\n\n--- golden\n+++ generated\n%s",
			unifiedDiff(string(golden), string(generated)))
	}
}

func TestNoOptionMethodExcluded(t *testing.T) {
	pluginBin := findPluginBinary(t)
	protocBin := findProtoc(t)

	gogoProtoRoot := modDir(t, "github.com/gogo/protobuf")
	vtprotoInclude := filepath.Join(modDir(t, "github.com/planetscale/vtprotobuf"), "include")
	vtpoolProtoDir := filepath.Join(repoRoot(t), "pkg", "vtproto", "gen")
	testdataDir := filepath.Join(vtpoolProtoDir, "test", "testdata")

	outDir := t.TempDir()

	cmd := exec.Command(protocBin,
		"--go-grpc-vtpool_out="+outDir,
		"--go-grpc-vtpool_opt=paths=source_relative",
		"--go-grpc-vtpool_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb",
		"-I="+testdataDir,
		"-I="+filepath.Join(gogoProtoRoot, "protobuf"),
		"-I="+vtprotoInclude,
		"-I="+vtpoolProtoDir,
		filepath.Join(testdataDir, "test.proto"),
	)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(pluginBin)+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "test_grpc_vtpool.pb.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	content := string(generated)

	for _, excluded := range []string{"NoOptionMethod", "PlainMethod"} {
		if strings.Contains(content, excluded) {
			t.Errorf("generated output should not contain %q but does", excluded)
		}
	}

	for _, included := range []string{
		"DeferRequestFromVTPool",
		"CallerRequestFromVTPool",
		"RefcountRequestFromVTPool",
		"defer in.ReturnToVTPool()",
		"defer m.ReturnToVTPool()",
		"_TestService_DeferMethod_VTPoolHandler",
		"_TestService_CallerMethod_VTPoolHandler",
		"_TestService_RefcountMethod_VTPoolHandler",
		"_TestService_DeferStream_VTPoolHandler",
		"_TestService_CallerStream_VTPoolHandler",
		"vtproto.WithReturnWG",
		"sync.WaitGroup",
	} {
		if !strings.Contains(content, included) {
			t.Errorf("generated output should contain %q but does not", included)
		}
	}
}

func TestCallerModeNoDefer(t *testing.T) {
	pluginBin := findPluginBinary(t)
	protocBin := findProtoc(t)

	gogoProtoRoot := modDir(t, "github.com/gogo/protobuf")
	vtprotoInclude := filepath.Join(modDir(t, "github.com/planetscale/vtprotobuf"), "include")
	vtpoolProtoDir := filepath.Join(repoRoot(t), "pkg", "vtproto", "gen")
	testdataDir := filepath.Join(vtpoolProtoDir, "test", "testdata")

	outDir := t.TempDir()

	cmd := exec.Command(protocBin,
		"--go-grpc-vtpool_out="+outDir,
		"--go-grpc-vtpool_opt=paths=source_relative",
		"--go-grpc-vtpool_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb",
		"-I="+testdataDir,
		"-I="+filepath.Join(gogoProtoRoot, "protobuf"),
		"-I="+vtprotoInclude,
		"-I="+vtpoolProtoDir,
		filepath.Join(testdataDir, "test.proto"),
	)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(pluginBin)+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "test_grpc_vtpool.pb.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	// The CallerMethod handler should NOT have "defer in.ReturnToVTPool()"
	// but SHOULD have "in.ReturnToVTPool()" inside the error branch.
	lines := strings.Split(string(generated), "\n")
	inCallerHandler := false
	for _, line := range lines {
		if strings.Contains(line, "_TestService_CallerMethod_VTPoolHandler") {
			inCallerHandler = true
		}
		if inCallerHandler && strings.TrimSpace(line) == "}" && !strings.Contains(line, "})") {
			break
		}
		if inCallerHandler && strings.Contains(line, "defer") && strings.Contains(line, "ReturnToVTPool") {
			t.Error("CallerMethod handler should not use defer for ReturnToVTPool")
		}
	}
}

func TestRefcountModeWaitGroupAndContext(t *testing.T) {
	pluginBin := findPluginBinary(t)
	protocBin := findProtoc(t)

	gogoProtoRoot := modDir(t, "github.com/gogo/protobuf")
	vtprotoInclude := filepath.Join(modDir(t, "github.com/planetscale/vtprotobuf"), "include")
	vtpoolProtoDir := filepath.Join(repoRoot(t), "pkg", "vtproto", "gen")
	testdataDir := filepath.Join(vtpoolProtoDir, "test", "testdata")

	outDir := t.TempDir()

	cmd := exec.Command(protocBin,
		"--go-grpc-vtpool_out="+outDir,
		"--go-grpc-vtpool_opt=paths=source_relative",
		"--go-grpc-vtpool_opt=Mgoogle/protobuf/descriptor.proto=google.golang.org/protobuf/types/descriptorpb",
		"-I="+testdataDir,
		"-I="+filepath.Join(gogoProtoRoot, "protobuf"),
		"-I="+vtprotoInclude,
		"-I="+vtpoolProtoDir,
		filepath.Join(testdataDir, "test.proto"),
	)
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(pluginBin)+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "test_grpc_vtpool.pb.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	lines := strings.Split(string(generated), "\n")
	inRefcountHandler := false
	foundWaitGroup := false
	foundWithReturnWG := false
	foundDeferDone := false
	foundDeferReturn := false
	for _, line := range lines {
		if strings.Contains(line, "func _TestService_RefcountMethod_VTPoolHandler") {
			inRefcountHandler = true
		}
		if inRefcountHandler && strings.TrimSpace(line) == "}" && !strings.Contains(line, "})") {
			break
		}
		if inRefcountHandler {
			if strings.Contains(line, "sync.WaitGroup") {
				foundWaitGroup = true
			}
			if strings.Contains(line, "WithReturnWG") {
				foundWithReturnWG = true
			}
			if strings.Contains(line, "defer wg.Done()") {
				foundDeferDone = true
			}
			if strings.Contains(line, "defer") && strings.Contains(line, "ReturnToVTPool") {
				foundDeferReturn = true
			}
		}
	}

	if !foundWaitGroup {
		t.Error("RefcountMethod handler should contain sync.WaitGroup")
	}
	if !foundWithReturnWG {
		t.Error("RefcountMethod handler should contain WithReturnWG")
	}
	if !foundDeferDone {
		t.Error("RefcountMethod handler should contain defer wg.Done()")
	}
	if foundDeferReturn {
		t.Error("RefcountMethod handler should NOT use defer in.ReturnToVTPool()")
	}
}

// --- helpers ---

func repoRoot(t *testing.T) string {
	t.Helper()
	// Walk up from this test file to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

func findPluginBinary(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "protoc-gen-go-grpc-vtpool")
	cmd := exec.Command("go", "build", "-o", bin, "./pkg/vtproto/gen/")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building plugin: %v\n%s", err, out)
	}
	return bin
}

func findProtoc(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "go", "bin", "protoc-3.20.1"),
		"protoc",
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	t.Skip("protoc not found in PATH")
	return ""
}

func modDir(t *testing.T, mod string) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", mod)
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolving module %s: %v", mod, err)
	}
	return strings.TrimSpace(string(out))
}

func unifiedDiff(a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")
	var diff strings.Builder
	maxLen := len(aLines)
	if len(bLines) > maxLen {
		maxLen = len(bLines)
	}
	for i := 0; i < maxLen; i++ {
		aLine, bLine := "", ""
		if i < len(aLines) {
			aLine = aLines[i]
		}
		if i < len(bLines) {
			bLine = bLines[i]
		}
		if aLine != bLine {
			diff.WriteString("- " + aLine + "\n")
			diff.WriteString("+ " + bLine + "\n")
		}
	}
	return diff.String()
}
