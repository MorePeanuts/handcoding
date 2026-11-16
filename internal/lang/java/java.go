// Package java runs native LeetCode Java solutions with a local JDK.
package java

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

const MinVersion = "17"
const DefaultRelease = 25

//go:embed runtime/*
var sources embed.FS

type Language struct {
	Home       string
	Release    int
	JVMArgs    []string
	BuildCache string
}

func (Language) ID() string                             { return "java" }
func (Language) Name() string                           { return "Java" }
func (Language) Aliases() []string                      { return nil }
func (Language) SolutionFile() string                   { return "Solution.java" }
func (Language) OrderedBaseline() bool                  { return true }
func (Language) Setup(context.Context, io.Writer) error { return nil }
func (l Language) release() int {
	if l.Release == 0 {
		return DefaultRelease
	}
	return l.Release
}
func CheckRelease(n int) error {
	if n < 17 {
		return fmt.Errorf("java.release must be 17 or later")
	}
	return nil
}

// JVM launch structure belongs to hc. Limit custom options to VM tuning and
// system properties, never a second entry point, class path, or argument file.
func CheckArgs(args []string) error {
	for _, a := range args {
		allowed := strings.HasPrefix(a, "-X") && !strings.HasPrefix(a, "-Xbootclasspath") || strings.HasPrefix(a, "-D") && !strings.HasPrefix(a, "-Djava.class.path=") || a == "-ea" || a == "-da"
		if strings.ContainsAny(a, "\x00\r\n") || !allowed {
			return fmt.Errorf("java.jvm_args: unsupported VM option %q (use -X/-XX tuning, -D properties, -ea or -da)", a)
		}
	}
	return nil
}
func (l Language) home(ctx context.Context) (string, error) {
	if l.Home != "" {
		return filepath.Abs(l.Home)
	}
	if h := os.Getenv("JAVA_HOME"); h != "" {
		return filepath.Abs(h)
	}
	b, err := lang.Probe(ctx, "java", nil, "-XshowSettings:properties", "-version")
	if err != nil {
		return "", fmt.Errorf("locate JDK: set java.home or JAVA_HOME: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), " = "); ok && k == "java.home" {
			return v, nil
		}
	}
	return "", fmt.Errorf("cannot locate java.home from the Java launcher; set java.home in hc.yaml")
}

var versionRE = regexp.MustCompile(`(?:version\s+"?|javac\s+)([0-9]+(?:\.[0-9]+)*)`)

func toolVersion(ctx context.Context, path string) (string, error) {
	b, err := lang.Probe(ctx, path, nil, "-version")
	if err != nil {
		return "", err
	}
	m := versionRE.FindSubmatch(b)
	if len(m) == 0 {
		return "", fmt.Errorf("cannot read Java version: %s", b)
	}
	return string(m[1]), nil
}
func (l Language) Toolchain() (lang.Toolchain, error) { return l.toolchain(context.Background()) }
func (l Language) toolchain(ctx context.Context) (lang.Toolchain, error) {
	if err := CheckRelease(l.release()); err != nil {
		return lang.Toolchain{}, err
	}
	if err := CheckArgs(l.JVMArgs); err != nil {
		return lang.Toolchain{}, err
	}
	h, err := l.home(ctx)
	if err != nil {
		return lang.Toolchain{}, err
	}
	java, javac := filepath.Join(h, "bin", "java"), filepath.Join(h, "bin", "javac")
	v, err := toolVersion(ctx, java)
	if err != nil {
		return lang.Toolchain{}, err
	}
	cv, err := toolVersion(ctx, javac)
	if err != nil {
		return lang.Toolchain{}, err
	}
	if strings.Split(v, ".")[0] != strings.Split(cv, ".")[0] {
		return lang.Toolchain{}, fmt.Errorf("JDK tools disagree: java %s, javac %s", v, cv)
	}
	if !lang.AtLeast(cv, strconv.Itoa(l.release())) {
		return lang.Toolchain{}, fmt.Errorf("JDK %s cannot compile java.release %d; select a newer JDK or lower java.release (protocol minimum is 17)", cv, l.release())
	}
	// Do not execute configured VM options during discovery: diagnostic
	// options such as -Xlog may write files even on a `java -version` probe.
	flags := []string{"--release", strconv.Itoa(l.release()), "javac=" + cv}
	flags = append(flags, l.JVMArgs...)
	for _, name := range []string{"JAVA_TOOL_OPTIONS", "JDK_JAVA_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVAC_OPTIONS"} {
		flags = append(flags, name+"="+os.Getenv(name))
	}
	// The runtime is part of the identity even on unversioned development builds.
	hash := sha256.New()
	_ = fs.WalkDir(sources, "runtime", func(p string, d fs.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			b, _ := sources.ReadFile(p)
			hash.Write(b)
		}
		return e
	})
	flags = append(flags, "runtime="+hex.EncodeToString(hash.Sum(nil)))
	return lang.Toolchain{Name: "Java", Version: v, Path: java, Min: MinVersion, Upgrade: "select a local JDK 17 or later with java.home", Flags: flags}, nil
}
func (l Language) Diagnose(ctx context.Context, o lang.DiagnosticOptions) []lang.Diagnostic {
	source := "system JDK"
	if l.Home != "" {
		source = "hc.yaml: java.home"
	} else if os.Getenv("JAVA_HOME") != "" {
		source = "JAVA_HOME"
	}
	h, err := l.home(ctx)
	checks := []lang.Diagnostic{}
	roles := []string{"java", "javac"}
	if o.Debug {
		roles = append(roles, "jdb")
	}
	for _, role := range roles {
		d := lang.Diagnostic{Language: "java", Role: role, Command: role, Source: source, Minimum: MinVersion}
		if err != nil {
			d.Status = "missing"
			d.Message = err.Error()
		} else {
			d.Command = filepath.Join(h, "bin", role)
			if d.Resolve() {
				if role == "jdb" {
					b, e := lang.Probe(ctx, d.Path, nil, "-version")
					d.Version = strings.TrimSpace(string(b))
					d.Status = "ok"
					if e != nil {
						d.Status = "error"
						d.Message = e.Error()
					}
				} else {
					v, e := toolVersion(ctx, d.Path)
					d.Version = v
					if e != nil {
						d.Status = "error"
						d.Message = e.Error()
					} else {
						d.CheckVersion()
					}
				}
			}
		}
		if role == "jdb" && o.Headless {
			d.Status = "unsupported"
			d.Message = "Java supports terminal debugging only"
		}
		checks = append(checks, d)
	}
	if err == nil && checks[0].Status == "ok" && checks[1].Status == "ok" {
		if _, e := l.toolchain(ctx); e != nil {
			checks[1].Status = "error"
			checks[1].Message = e.Error()
		}
	}
	return checks
}
func write(path string, b []byte) error {
	if old, e := os.ReadFile(path); e == nil && bytes.Equal(old, b) {
		return nil
	}
	if e := os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
		return e
	}
	return os.WriteFile(path, b, 0o644)
}
func (l Language) Prepare(dir string, p *problem.Problem) error {
	if err := CheckRelease(l.release()); err != nil {
		return err
	}
	for _, name := range []string{"HCJson.java", "HCRuntime.java", "HCTypes.java", "Pair.java"} {
		b, err := sources.ReadFile("runtime/" + name)
		if err != nil {
			return err
		}
		dest := name
		if name == "Pair.java" {
			dest = "javafx/util/Pair.java"
		}
		if err = write(filepath.Join(dir, "_hc_java", dest), b); err != nil {
			return err
		}
	}
	node := nodeTypes[p.NodeVariant()]
	if node == "" {
		node = "// This problem has no Node variant.\n"
	}
	if err := write(filepath.Join(dir, "_hc_java", "Node.java"), []byte("import java.util.*;\n"+node)); err != nil {
		return err
	}
	spec := map[string]any{"kind": p.Kind, "function": p.Function, "class": p.Class, "judge": map[string]any{"output": p.Judge.Output, "output_length": p.Judge.OutputLength}, "node": p.NodeVariant()}
	if rt := p.Judge.RoundTrip; rt != nil {
		spec["judge"].(map[string]any)["round_trip"] = map[string]string{"encode": rt.Encode, "decode": rt.Decode, "instances": rt.Instances}
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	// Java string literals safely embed JSON without interpreting its escapes.
	literal, _ := json.Marshal(string(b))
	src := "class HCRunner { public static void main(String[] args) throws Exception { HCRuntime.run(" + string(literal) + "); } }\n"
	if err = write(filepath.Join(dir, "_hc_java", "HCRunner.java"), []byte(src)); err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(abs))
	project := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><projectDescription><name>hc-java-%x</name><buildSpec><buildCommand><name>org.eclipse.jdt.core.javabuilder</name></buildCommand></buildSpec><natures><nature>org.eclipse.jdt.core.javanature</nature></natures></projectDescription>`, sum[:8])
	classpath := `<?xml version="1.0" encoding="UTF-8"?><classpath><classpathentry kind="src" path="" excluding="_hc_java/|.hc-java-bin/"/><classpathentry kind="src" path="_hc_java"/><classpathentry kind="con" path="org.eclipse.jdt.launching.JRE_CONTAINER/org.eclipse.jdt.internal.debug.ui.launcher.StandardVMType/JavaSE-` + strconv.Itoa(l.release()) + `"/><classpathentry kind="output" path=".hc-java-bin"/></classpath>`
	prefs := fmt.Sprintf("eclipse.preferences.version=1\norg.eclipse.jdt.core.compiler.compliance=%d\norg.eclipse.jdt.core.compiler.source=%d\norg.eclipse.jdt.core.compiler.codegen.targetPlatform=%d\norg.eclipse.jdt.core.compiler.release=enabled\n", l.release(), l.release(), l.release())
	for file, data := range map[string]string{".project": project, ".classpath": classpath, ".settings/org.eclipse.jdt.core.prefs": prefs} {
		if err = write(filepath.Join(dir, file), []byte(data+"\n")); err != nil {
			return err
		}
	}
	// Create the declared output directory so generated-file inventory and ignores agree.
	return os.MkdirAll(filepath.Join(dir, ".hc-java-bin"), 0o755)
}
func javaSources(dir string) ([]string, error) {
	paths := []string{filepath.Join(dir, "Solution.java")}
	err := filepath.WalkDir(filepath.Join(dir, "_hc_java"), func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.HasSuffix(p, ".java") {
			paths = append(paths, p)
		}
		return nil
	})
	return paths, err
}
func (l Language) Build(ctx context.Context, dir, work string) error {
	tc, err := l.toolchain(ctx)
	if err != nil {
		return err
	}
	files, err := javaSources(dir)
	if err != nil {
		return err
	}
	target := filepath.Join(work, "classes")
	cache := l.BuildCache != ""
	if cache {
		h := sha256.New()
		h.Write([]byte(tc.Path + tc.Version + strings.Join(tc.Flags, "\x00")))
		for _, p := range files {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			fmt.Fprintf(h, "%d:%s%d:", len(filepath.Base(p)), filepath.Base(p), len(b))
			h.Write(b)
		}
		target = filepath.Join(l.BuildCache, hex.EncodeToString(h.Sum(nil))[:32])
	}
	if _, e := os.Stat(filepath.Join(target, "complete")); e != nil || !cache {
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		tmp, e := os.MkdirTemp(filepath.Dir(target), ".java-build-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(tmp)
		args := []string{"--release", strconv.Itoa(l.release()), "-encoding", "UTF-8", "-g", "-proc:none", "-d", tmp}
		args = append(args, files...)
		cmd := exec.CommandContext(ctx, filepath.Join(filepath.Dir(tc.Path), "javac"), args...)
		cmd.Dir = dir
		if b, e := cmd.CombinedOutput(); e != nil {
			return &lang.CompileError{Output: string(b) + "\n" + e.Error()}
		}
		if err = write(filepath.Join(tmp, "complete"), []byte("ok\n")); err != nil {
			return err
		}
		if err = os.Rename(tmp, target); err != nil {
			if _, e = os.Stat(filepath.Join(target, "complete")); e != nil || !cache {
				// Cleanup can leave a hash directory containing unknown user
				// files. Keep it intact and use this run's private output.
				fallback, e := os.MkdirTemp(work, "classes-")
				if e != nil {
					return e
				}
				if e = os.CopyFS(fallback, os.DirFS(tmp)); e != nil {
					return e
				}
				target = fallback
			}
		}
	}
	if err = write(filepath.Join(work, "java"), []byte(tc.Path)); err != nil {
		return err
	}
	return write(filepath.Join(work, "classpath"), []byte(target))
}
func (l Language) Command(ctx context.Context, dir, work string) *exec.Cmd {
	launcher, _ := os.ReadFile(filepath.Join(work, "java"))
	b, _ := os.ReadFile(filepath.Join(work, "classpath"))
	args := []string{}
	args = append(args, l.JVMArgs...)
	args = append(args, "-cp", string(b), "HCRunner")
	cmd := exec.CommandContext(ctx, string(launcher), args...)
	cmd.Dir = dir
	return cmd
}
func (l Language) CheckMinVersion(ctx context.Context, p *problem.Problem, file string) error {
	dir, err := os.MkdirTemp("", "hc-java-min-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err = write(filepath.Join(dir, "Solution.java"), b); err != nil {
		return err
	}
	l.Release = 17
	l.BuildCache = ""
	if err = l.Prepare(dir, p); err != nil {
		return err
	}
	return l.Build(ctx, dir, dir)
}
func (Language) LeetCode(_ *problem.Problem, src string) (string, string, error) {
	return "java", src, nil
}
