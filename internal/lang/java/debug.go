package java

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/problem"
)

func (l Language) DebugCommand(dir, work string, p *problem.Problem, o lang.DebugOptions) (*exec.Cmd, error) {
	if o.Headless {
		return nil, fmt.Errorf("java supports terminal debugging only; omit --headless")
	}
	if err := l.Build(context.Background(), dir, work); err != nil {
		return nil, err
	}
	home, err := l.home(context.Background())
	if err != nil {
		return nil, err
	}
	debugger, err := lang.ResolveExecutable(filepath.Join(home, "bin", "jdb"))
	if err != nil {
		return nil, err
	}
	entry := "Solution." + p.EntryName()
	if p.Kind == problem.KindClass {
		entry = p.Class.Name + ".<init>"
	}
	if p.Judge.RoundTrip != nil {
		entry = p.Class.Name + "." + p.Judge.RoundTrip.Encode
	}
	// jdb reads commands from user.dir. Point only the debugger's property at
	// our temporary directory; the debuggee retains the solution's working dir.
	if err = write(filepath.Join(work, "jdb.ini"), []byte("stop in "+entry+"\nrun\n")); err != nil {
		return nil, err
	}
	cp, err := os.ReadFile(filepath.Join(work, "classpath"))
	if err != nil {
		return nil, err
	}
	// Java 17 jdb cannot forward -R options. A Java launcher argument file
	// also preserves VM arguments containing spaces, commas and quotes.
	var vm strings.Builder
	for _, a := range append(append([]string{}, l.JVMArgs...), "-cp", string(cp)) {
		vm.WriteString(strconv.Quote(a))
		vm.WriteByte('\n')
	}
	argfile := filepath.Join(work, "java.args")
	if err = write(argfile, []byte(vm.String())); err != nil {
		return nil, err
	}
	connector := "com.sun.jdi.CommandLineLaunch:options=" + strconv.Quote("@"+argfile) + ",main=HCRunner"
	args := []string{"-J-Duser.dir=" + work, "-sourcepath", dir, "-connect", connector}
	cmd := exec.Command(debugger, args...)
	cmd.Dir = dir
	return cmd, nil
}
