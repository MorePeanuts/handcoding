package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/problem"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

func newFetchCmd(root *rootOptions) *cobra.Command {
	var (
		siteName string
		force    bool
	)
	cmd := &cobra.Command{
		Use:     "fetch <slug|id|url>...",
		Aliases: []string{"pull"},
		Short:   "Import problems from LeetCode",
		Long: `Download problems from LeetCode's public API into the workspace's own
problems (packs/local/problems/, or problems/ in a pack checkout): statement
(with its images, for offline use), signature, Go/Python templates and the
examples (with expected outputs when they can be parsed from the statement).
Premium problems are not available.

The statements written are those of the pack's locales (locales in hc.yaml
for your own problems, [en] unless set; in pack.yaml in a pack checkout).
Both sites have the English statement; only leetcode.cn has the Chinese one
(zh-CN), so a pack whose default locale is zh-CN fetches from leetcode.cn.
Fetching a problem again from the other site adds the locales it lacks.

The problem is not ready to submit yet: LeetCode publishes only the examples
and not how answers are judged (any order? several valid answers?).
` + "`hc test`" + ` shows your output next to the examples meanwhile. Run
` + "`hc gen <slug>`" + ` to let an agent write the full test suite and decide the
judge rule, or set judge.compare by hand.

  hc fetch two-sum 3 146
  hc fetch --site cn lru-cache
  hc fetch https://leetcode.cn/problems/two-sum/description/
  hc fetch --site cn "LCR 001"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := root.openApp()
			if err != nil {
				return err
			}
			defer a.Close()
			if siteName == "" {
				siteName = a.WS.Config.LeetCode.Site
			}
			site, err := leetcode.SiteByName(siteName)
			if err != nil {
				return err
			}
			clients := map[string]*leetcode.Client{}
			var failed int
			for _, ref := range args {
				// A URL names its own site.
				refSite := site
				if s, slug, ok := leetcode.ParseURL(ref); ok {
					refSite, ref = s, slug
				}
				client := clients[refSite.Name]
				if client == nil {
					client = leetcode.NewClient(refSite, a.WS.StatePath())
					clients[refSite.Name] = client
				}
				if err := fetchOne(cmd, a, client, ref, force); err != nil {
					fmt.Fprintf(os.Stderr, "%s %s: %v\n", ui.Red.Render("error"), ref, err)
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d problem(s) failed", failed, len(args))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&siteName, "site", "", "com or cn (default from hc.yaml)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing problem (keeps nothing)")
	return cmd
}

func fetchOne(cmd *cobra.Command, a *app.App, client *leetcode.Client, ref string, force bool) error {
	ctx := cmd.Context()
	k := a.WS.WritablePack()
	if k.Err != nil {
		return k.Err
	}
	locales, def := k.Manifest.Locales, k.Manifest.DefaultLocale()
	site := "leetcode." + client.Site.Name
	slug, err := client.ResolveSlug(ctx, ref)
	if err != nil {
		return err
	}
	q, err := client.Question(ctx, slug)
	if err != nil {
		return err
	}
	// LeetCode accepts slugs in any case; compare with its canonical one.
	existing, err := packProblem(k, q.TitleSlug)
	if err != nil {
		return err
	}
	conv, err := leetcode.Convert(q, client.Site)
	if err != nil {
		return err
	}
	if existing != nil && !force {
		return addLocales(cmd, a, client, existing, conv, locales, def)
	}
	missing := conv.Localize(locales, def)
	if slices.Contains(missing, def) {
		return fmt.Errorf("%s has no %s statement of %s, and %s is the default locale of pack %s%s",
			site, def, q.TitleSlug, def, k.Name, otherSite(client, conv, def))
	}
	client.DownloadImages(ctx, conv)
	// Only drop the old copy once the new one is ready to be written.
	if existing != nil {
		if err := os.RemoveAll(existing.Dir); err != nil {
			return err
		}
	}
	dir := a.WS.NewProblemDir(conv.Problem)
	if err := conv.Save(dir); err != nil {
		return err
	}
	withExpected := 0
	for _, c := range conv.Cases {
		if c.HasExpected() {
			withExpected++
		}
	}
	fmt.Printf("%s %s. %s → %s (%d examples, %d with expected output)\n", ui.Green.Render("fetched"),
		conv.Problem.ID, conv.Problem.Title, a.WS.Rel(dir), len(conv.Cases), withExpected)
	for _, l := range missing {
		conv.Warnings = append(conv.Warnings, fmt.Sprintf("no %s statement on %s%s", l, site, otherSite(client, conv, l)))
	}
	for _, w := range conv.Warnings {
		fmt.Printf("    %s %s\n", ui.Yellow.Render("note:"), w)
	}
	conv.Problem.Pack = k.Name
	fmt.Printf("    next: hc gen %s — writes the test cases and decides how answers are judged\n", a.WS.Ref(conv.Problem))
	return nil
}

// addLocales adds to p, a problem fetched before, the statements of locales
// it lacks and conv has, leaving the rest of p as it is. Without any to add,
// p is already fetched.
func addLocales(cmd *cobra.Command, a *app.App, client *leetcode.Client, p *problem.Problem, conv *leetcode.Converted, locales []string, def string) error {
	var lacking, add []string
	for _, l := range locales {
		if _, err := os.Stat(filepath.Join(p.Dir, problem.StatementFile(l, def))); errors.Is(err, os.ErrNotExist) {
			lacking = append(lacking, l)
			if conv.Has(l) {
				add = append(add, l)
			}
		}
	}
	if len(add) == 0 {
		msg := fmt.Sprintf("already fetched into %s (use --force to replace)", a.WS.Rel(p.Dir))
		if len(lacking) > 0 {
			msg += fmt.Sprintf("; it lacks %s, which leetcode.%s has no statement in either", strings.Join(lacking, ", "), client.Site.Name)
		}
		return errors.New(msg)
	}
	conv.Localize(add, def)
	existing, err := readAssets(p.Dir)
	if err != nil {
		return err
	}
	conv.Existing = existing
	client.DownloadImages(cmd.Context(), conv)
	if err := conv.AddTo(p); err != nil {
		return err
	}
	fmt.Printf("%s %s statement(s) to %s → %s\n", ui.Green.Render("added"), strings.Join(add, ", "), a.WS.Ref(p), a.WS.Rel(p.Dir))
	for _, w := range conv.Warnings {
		fmt.Printf("    %s %s\n", ui.Yellow.Render("note:"), w)
	}
	return nil
}

// otherSite suggests the site to fetch the statement in locale from, when
// the client's site has none and the other one may: leetcode.cn translates
// statements into Chinese, and leetcode.com has the English ones of
// problems leetcode.cn has only in Chinese.
func otherSite(client *leetcode.Client, conv *leetcode.Converted, locale string) string {
	switch {
	case client.Site.Name == leetcode.SiteCom.Name && strings.EqualFold(locale, leetcode.LocaleChinese):
		return fmt.Sprintf(": fetch %s from leetcode.cn to add it", conv.Problem.Slug)
	case !strings.EqualFold(locale, leetcode.LocaleEnglish) && !strings.EqualFold(locale, leetcode.LocaleChinese):
		return fmt.Sprintf(" (LeetCode has statements in %s and %s only)", leetcode.LocaleEnglish, leetcode.LocaleChinese)
	}
	return ""
}

// readAssets reads the files in assets/ of the problem in dir, by their path
// in it.
func readAssets(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	entries, err := os.ReadDir(filepath.Join(dir, problem.AssetsDir))
	if errors.Is(err, os.ErrNotExist) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, problem.AssetsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[problem.AssetsDir+"/"+e.Name()] = data
	}
	return files, nil
}
