package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/lang"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

func newLeetCodeCmd(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leetcode",
		Short: "Run and submit solutions on your LeetCode account",
		Long: `Run a solution on LeetCode, or submit it to your account, as on the
problem's page. Problems come from LeetCode (hc fetch, or a pack such as the
official leetcode pack); the site and slug are those of source in problem.yaml.

Log in first with hc leetcode login. hc keeps the session in your user
configuration directory, never in the workspace. LeetCode results are not
recorded in hc's history.`,
	}
	cmd.AddCommand(newLeetCodeLoginCmd(root), newLeetCodeLogoutCmd(), newLeetCodeTestCmd(root), newLeetCodeSubmitCmd(root))
	return cmd
}

// defaultSite is the site of --site, else leetcode.site of the workspace's
// hc.yaml when there is a workspace, else leetcode.com.
func defaultSite(root *rootOptions, name string) (leetcode.Site, error) {
	if name == "" {
		if a, err := root.openApp(); err == nil {
			name = a.WS.Config.LeetCode.Site
			a.Close()
		}
	}
	return leetcode.SiteByName(name)
}

// browserLoginTimeout is how long hc leetcode login waits for the browser.
const browserLoginTimeout = 5 * time.Minute

func newLeetCodeLoginCmd(root *rootOptions) *cobra.Command {
	var (
		siteName string
		paste    bool
		port     int
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to LeetCode with your browser's session",
		Long: `Log in to leetcode.com or leetcode.cn with the session of a browser signed
in to it. LeetCode signs in with a CAPTCHA, so hc cannot log in with a
password: it takes the browser's LEETCODE_SESSION cookie instead.

By default hc opens the site's authorization page, as LeetCode's VS Code
extension does; sign in there if the browser is not signed in yet. LeetCode
then sends the browser's cookies to hc, on a port of 127.0.0.1 (any free one,
or --port). Over SSH, forward that port (ssh -L 8765:127.0.0.1:8765 host,
with --port 8765) and open the page on your own machine.

With --paste, or with standard input not a terminal, hc reads the session
instead. Open the site signed in and the developer tools (F12), then either

  - under Application (Chrome, Edge) or Storage (Firefox, Safari) → Cookies,
    copy the value of LEETCODE_SESSION, or
  - under Network, reload the page, and copy any request to the site as cURL
    (right-click → Copy → Copy as cURL), or copy its Cookie request header,

and paste it (nothing is shown as you paste), or pipe it in: on macOS, copy
it and run pbpaste | hc leetcode login.

hc checks the session with LeetCode before saving it, readable by you only,
in hc/leetcode.json of your user configuration directory. A session lasts a
few weeks; log in again when LeetCode refuses it.

  hc leetcode login --site cn
  hc leetcode login --site cn --paste`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := defaultSite(root, siteName)
			if err != nil {
				return err
			}
			c := leetcode.NewClient(site, "")
			var user, path string
			if paste || !stdinIsTerminal(cmd) {
				text, err := readSession(cmd)
				if err != nil {
					return err
				}
				s, err := leetcode.ParseSession(text)
				if err != nil {
					return err
				}
				user, path, err = app.LeetCodeLogin(cmd.Context(), c, s)
				if err != nil {
					return err
				}
			} else if user, path, err = browserLogin(cmd, c, port); err != nil {
				return err
			}
			fmt.Printf("%s to leetcode.%s as %s (session kept in %s)\n", ui.Green.Render("logged in"), site.Name, user, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&siteName, "site", "", "com or cn (default from hc.yaml)")
	cmd.Flags().BoolVar(&paste, "paste", false, "paste the session instead of authorizing in the browser")
	cmd.Flags().IntVar(&port, "port", 0, "port of 127.0.0.1 the browser sends the session to (default: any free one)")
	cmd.MarkFlagsMutuallyExclusive("paste", "port")
	return cmd
}

// browserLogin logs c in through the browser, opening the site's
// authorization page when this machine has a desktop.
func browserLogin(cmd *cobra.Command, c *leetcode.Client, port int) (user, path string, err error) {
	b, err := app.StartLeetCodeBrowserLogin(c, port)
	if err != nil {
		return "", "", err
	}
	defer b.Close()
	fmt.Fprintf(os.Stderr, "Authorize hc in a browser signed in to leetcode.%s, at\n\n  %s\n\n", c.Site.Name, b.URL)
	switch {
	case overSSH():
		fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("Over SSH: on your own machine, forward the port (ssh -L %d:127.0.0.1:%d <this host>) and open the page there; --port picks the port.", b.Port, b.Port)))
	case openURL(b.URL) == nil:
		fmt.Fprintln(os.Stderr, ui.Faint.Render("Opened it in your browser."))
	}
	fmt.Fprintln(os.Stderr, ui.Faint.Render("Waiting for the browser (Ctrl-C to stop; --paste pastes the session instead)…"))
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, browserLoginTimeout)
	defer cancel()
	user, path, err = b.Wait(ctx)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		err = fmt.Errorf("no reply from the browser in %v; run hc leetcode login again, or with --paste", browserLoginTimeout)
	case errors.Is(err, context.Canceled):
		err = errors.New("login canceled")
	}
	return user, path, err
}

func stdinIsTerminal(cmd *cobra.Command) bool {
	f, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// overSSH reports whether hc runs in an SSH session, where a browser it
// opened would not be the user's.
func overSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// openURL opens url in the desktop's browser. Without a desktop (no display
// on Linux) it does nothing and fails, rather than start a text browser in
// the terminal.
func openURL(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return errors.New("no display")
		}
		c = exec.Command("xdg-open", url)
	}
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }() // reap it; its exit status does not matter
	return nil
}

// readSession reads what holds the session: on a terminal, a line read
// without echo, and the lines after it while each ends with a backslash, as
// a pasted cURL command's do (left unread, they would run in the shell);
// else all of standard input.
func readSession(cmd *cobra.Command) (string, error) {
	f, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
		if err != nil {
			return "", fmt.Errorf("reading the session: %w", err)
		}
		return string(b), nil
	}
	fmt.Fprint(os.Stderr, "LEETCODE_SESSION, a Cookie header or a cURL command: ")
	defer fmt.Fprintln(os.Stderr)
	var text strings.Builder
	for {
		line, err := term.ReadPassword(int(f.Fd()))
		if err != nil {
			return "", fmt.Errorf("reading the session: %w", err)
		}
		text.Write(line)
		text.WriteByte('\n')
		if !strings.HasSuffix(strings.TrimSpace(string(line)), `\`) {
			return text.String(), nil
		}
	}
}

func newLeetCodeLogoutCmd() *cobra.Command {
	var siteName string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Forget the LeetCode sessions hc saved",
		Long: `Forget the LeetCode sessions hc saved: of every site, or of the site of
--site. Only hc forgets them: LeetCode keeps a session valid until it expires,
so log out in the browser too to end it there.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var sites []leetcode.Site
			if siteName != "" {
				site, err := leetcode.SiteByName(siteName)
				if err != nil {
					return err
				}
				sites = append(sites, site)
			}
			removed, err := app.LeetCodeLogout(sites...)
			if err != nil {
				return err
			}
			for _, name := range removed {
				fmt.Printf("%s of leetcode.%s\n", ui.Green.Render("logged out"), name)
			}
			if len(removed) > 0 {
				return nil
			}
			if len(sites) == 0 {
				fmt.Println("Not logged in to LeetCode.")
				return nil
			}
			msg := fmt.Sprintf("Not logged in to leetcode.%s", sites[0].Name)
			if others, err := app.LeetCodeSites(); err == nil && len(others) > 0 {
				msg += " (logged in to leetcode." + strings.Join(others, ", leetcode.") + ")"
			}
			fmt.Println(msg + ".")
			return nil
		},
	}
	cmd.Flags().StringVar(&siteName, "site", "", "com or cn (default: every site)")
	return cmd
}

type leetCodeOptions struct {
	lang, site, input string
	verbose           bool
}

// withLeetCode resolves the problem, its language and where it is on
// LeetCode, and signs a client in to that site.
func withLeetCode(root *rootOptions, ref string, o *leetCodeOptions, fn func(a *app.App, p *problem.Problem, l lang.Language, t app.LeetCodeTarget, c *leetcode.Client) error) error {
	return root.withProblem(ref, o.lang, func(a *app.App, p *problem.Problem, l lang.Language) error {
		t, err := app.LeetCodeProblem(p, o.site)
		if err != nil {
			return err
		}
		sess, err := app.LeetCodeSession(t.Site)
		if err != nil {
			return err
		}
		c := leetcode.NewClient(t.Site, a.WS.StatePath())
		c.Session = sess
		return fn(a, p, l, t, c)
	})
}

// leetCodeErr adds what to do to LeetCode's refusals.
func leetCodeErr(err error, site leetcode.Site) error {
	if errors.Is(err, leetcode.ErrBlocked) {
		return fmt.Errorf("%w; if the session has expired, run `hc leetcode login --site %s`", err, site.Name)
	}
	return err
}

func newLeetCodeTestCmd(root *rootOptions) *cobra.Command {
	o := &leetCodeOptions{}
	cmd := &cobra.Command{
		Use:   "test <problem>",
		Short: "Run the solution on LeetCode",
		Long: `Run your solution on LeetCode, like Run on the problem's page: on its example
testcases, or on --input, testcases in LeetCode's format (one JSON value per
line, a line per parameter, cases one after another). Each case shows your
output next to LeetCode's expected answer.

  hc leetcode test two-sum
  hc leetcode test two-sum --input $'[3,3]\n6'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLeetCode(root, args[0], o, func(a *app.App, p *problem.Problem, l lang.Language, t app.LeetCodeTarget, c *leetcode.Client) error {
				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
				defer stop()
				fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("Running %s on leetcode.%s [%s]…", t.Slug, t.Site.Name, l.Name())))
				run, err := a.LeetCodeTest(ctx, c, p, l, t.Slug, o.input)
				if err != nil {
					return leetCodeErr(err, t.Site)
				}
				if !printLeetCodeTest(p, run, o.verbose) {
					return errSilent
				}
				return nil
			})
		},
	}
	langFlag(cmd, &o.lang)
	cmd.Flags().StringVar(&o.site, "site", "", "com or cn (default: the problem's source)")
	cmd.Flags().StringVarP(&o.input, "input", "i", "", "testcases in LeetCode's format (default: the examples)")
	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", false, "show inputs and outputs in full")
	return cmd
}

func newLeetCodeSubmitCmd(root *rootOptions) *cobra.Command {
	o := &leetCodeOptions{}
	cmd := &cobra.Command{
		Use:   "submit <problem>",
		Short: "Submit the solution to your LeetCode account",
		Long: `Submit your solution to LeetCode, like Submit on the problem's page, and show
LeetCode's verdict. When it fails, the case it failed on is shown with your
output and the expected one; for a function problem whose input LeetCode gives
as it is, also the hc cases add command that adds it to the problem's cases.

  hc leetcode submit two-sum -l python`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLeetCode(root, args[0], o, func(a *app.App, p *problem.Problem, l lang.Language, t app.LeetCodeTarget, c *leetcode.Client) error {
				ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
				defer stop()
				fmt.Fprintln(os.Stderr, ui.Faint.Render(fmt.Sprintf("Submitting %s to leetcode.%s [%s]…", t.Slug, t.Site.Name, l.Name())))
				run, err := a.LeetCodeSubmit(ctx, c, p, l, t.Slug)
				if err != nil {
					return leetCodeErr(err, t.Site)
				}
				if !printLeetCodeSubmit(a, p, run, o.verbose) {
					return errSilent
				}
				return nil
			})
		},
	}
	langFlag(cmd, &o.lang)
	cmd.Flags().StringVar(&o.site, "site", "", "com or cn (default: the problem's source)")
	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", false, "show the failing case in full")
	return cmd
}

// leetCodeClip is how much of an input or output is shown without -v.
const leetCodeClip = 300

// printLeetCodeTest prints a run and reports whether every case matched.
func printLeetCodeTest(p *problem.Problem, run *app.LeetCodeRun, verbose bool) bool {
	r := run.Result
	show := func(label, s string) {
		if !verbose {
			s = clip(s, leetCodeClip)
		}
		fmt.Printf("    %s %s\n", ui.Faint.Render(fmt.Sprintf("%-9s", label)), strings.ReplaceAll(s, "\n", "\n              "))
	}
	if e := r.Compile(); e != "" {
		fmt.Println(ui.Red.Render("✘ " + statusMsg(r, "Compile Error")))
		fmt.Println(indent(e))
		return false
	}
	if !r.RunSuccess {
		fmt.Println(ui.Red.Render("✘ " + statusMsg(r, "Runtime Error")))
		if e := r.Runtime(); e != "" {
			fmt.Println(indent(e))
		}
		if r.LastTestcase != "" {
			show("Input", string(r.LastTestcase))
		}
		return false
	}
	n := len(r.CompareResult)
	if n == 0 {
		n = len(r.CodeAnswer)
	}
	inputs := splitTestcases(run.Input, len(p.InputKeys()), n)
	at := func(l leetcode.Lines, i int) string {
		if i < len(l) {
			return l[i]
		}
		return ""
	}
	matched := 0
	for i := range n {
		ok := i < len(r.CompareResult) && r.CompareResult[i] == '1'
		if ok {
			matched++
			fmt.Printf("%s\n", ui.Green.Render(fmt.Sprintf("✔ Case %d", i+1)))
		} else {
			fmt.Printf("%s %s\n", ui.Red.Render(fmt.Sprintf("✘ Case %d", i+1)), ui.Red.Render("Wrong Answer"))
		}
		if inputs != nil {
			show("Input", inputs[i])
		}
		show("Output", at(r.CodeAnswer, i))
		show("Expected", at(r.ExpectedCodeAnswer, i))
		if out := strings.TrimRight(at(r.StdOutputList, i), "\n"); out != "" {
			show("Stdout", out)
		}
	}
	if inputs == nil {
		show("Input", run.Input)
	}
	fmt.Println()
	if matched == n && r.CorrectAnswer {
		fmt.Printf("%s · %d/%d match LeetCode's answers\n", ui.Green.Render("Accepted"), matched, n)
		return true
	}
	fmt.Printf("%s · %d/%d match LeetCode's answers\n", ui.Red.Render("Wrong Answer"), matched, n)
	return false
}

// splitTestcases splits LeetCode testcases into n cases of k lines each, or
// returns nil when they are not that.
func splitTestcases(input string, k, n int) []string {
	lines := strings.Split(strings.TrimRight(input, "\r\n"), "\n")
	if k <= 0 || n <= 0 || len(lines) != k*n {
		return nil
	}
	out := make([]string, n)
	for i := range n {
		out[i] = strings.Join(lines[i*k:(i+1)*k], "\n")
	}
	return out
}

// printLeetCodeSubmit prints a submission's verdict and reports whether it
// was accepted.
func printLeetCodeSubmit(a *app.App, p *problem.Problem, run *app.LeetCodeRun, verbose bool) bool {
	r := run.Result
	counts := ""
	if r.TotalTestcases > 0 {
		counts = fmt.Sprintf(" · %d/%d testcases passed", r.TotalCorrect, r.TotalTestcases)
	}
	if r.Accepted() {
		perf := ""
		if r.StatusRuntime != "" {
			perf += fmt.Sprintf(" · runtime %s%s", r.StatusRuntime, beats(r.RuntimePercentile))
		}
		if r.StatusMemory != "" {
			perf += fmt.Sprintf(" · memory %s%s", r.StatusMemory, beats(r.MemoryPercentile))
		}
		fmt.Printf("%s%s%s\n", ui.Green.Render("✔ Accepted"), counts, perf)
		return true
	}
	fmt.Printf("%s%s\n", ui.Red.Render("✘ "+statusMsg(r, "Not accepted")), counts)
	if e := r.Compile(); e != "" {
		fmt.Println(indent(e))
		return false
	}
	if e := r.Runtime(); e != "" {
		fmt.Println(indent(e))
	}
	show := func(label, s string) {
		if s == "" {
			return
		}
		if !verbose {
			s = clip(s, leetCodeClip)
		}
		fmt.Printf("    %s %s\n", ui.Faint.Render(fmt.Sprintf("%-9s", label)), strings.ReplaceAll(s, "\n", "\n              "))
	}
	show("Input", strings.TrimRight(string(r.LastTestcase), "\n"))
	show("Output", string(r.CodeOutput))
	show("Expected", string(r.ExpectedOutput))
	show("Stdout", strings.TrimRight(string(r.StdOutput), "\n"))
	if run.Case != nil {
		printCaseSuggestion(a, p, *run.Case, statusMsg(r, "not accepted"), verbose)
	}
	return false
}

// printCaseSuggestion prints the hc cases add command that adds c, the case
// a submission failed on, to the problem.
func printCaseSuggestion(a *app.App, p *problem.Problem, c problem.Case, status string, verbose bool) {
	if line, err := problem.MarshalCase(c); err != nil || len(line) > problem.MaxStoredCase {
		fmt.Printf("\n%s the failing case is over %d KB, too large to store; build such inputs with %s instead\n",
			ui.Faint.Render("note:"), problem.MaxStoredCase>>10, problem.GeneratorFile)
		return
	}
	input, err := compactJSON(c.Input)
	if err != nil {
		return
	}
	cmd := fmt.Sprintf("hc cases add %s --input %s", a.WS.Ref(p), shellQuote(input))
	if c.HasExpected() {
		cmd += " --expected " + shellQuote(string(c.Expected))
	}
	cmd += " --note " + shellQuote("from LeetCode: "+status)
	if !verbose && len(cmd) > 2000 {
		fmt.Printf("\n%s run with -v for the hc cases add command that adds this case\n", ui.Faint.Render("note:"))
		return
	}
	fmt.Printf("\n%s\n  %s\n", ui.Faint.Render("Add it to the problem's cases:"), cmd)
}

func compactJSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func statusMsg(r *leetcode.Result, fallback string) string {
	if r.StatusMsg != "" {
		return r.StatusMsg
	}
	return fallback
}

func beats(pct float64) string {
	if pct <= 0 {
		return ""
	}
	return fmt.Sprintf(" (beats %.1f%%)", pct)
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}
