package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/MorePeanuts/handcoding/internal/app"
	"github.com/MorePeanuts/handcoding/internal/leetcode"
	"github.com/MorePeanuts/handcoding/internal/ui"
)

func newFetchCmd(root *rootOptions) *cobra.Command {
	var (
		siteName   string
		translated bool
		force      bool
	)
	cmd := &cobra.Command{
		Use:     "fetch <slug|id|url>...",
		Aliases: []string{"pull"},
		Short:   "Import problems from LeetCode",
		Long: `Download problems from LeetCode's public API into the bank: statement
(with its images, for offline use), signature, Go/Python templates and the
examples (with expected outputs when they can be parsed from the statement).
Premium problems are not available.

The problem is not ready to submit yet: LeetCode publishes only the examples
and not how answers are judged (any order? several valid answers?).
` + "`hc test`" + ` shows your output next to the examples meanwhile. Run
` + "`hc gen <slug>`" + ` to let an agent write the full test suite and decide the
judge rule, or set judge.compare by hand.

  hc fetch two-sum 3 146
  hc fetch --site cn --translated lru-cache
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
				tr := translated
				if !cmd.Flags().Changed("translated") {
					tr = refSite == leetcode.SiteCN
				}
				client := clients[refSite.Name]
				if client == nil {
					client = leetcode.NewClient(refSite, a.WS.StatePath())
					clients[refSite.Name] = client
				}
				if err := fetchOne(cmd, a, client, ref, tr, force); err != nil {
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
	cmd.Flags().BoolVar(&translated, "translated", false, "use the translated statement (default on for cn)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing problem (keeps nothing)")
	return cmd
}

func fetchOne(cmd *cobra.Command, a *app.App, client *leetcode.Client, ref string, translated, force bool) error {
	ctx := cmd.Context()
	slug, err := client.ResolveSlug(ctx, ref)
	if err != nil {
		return err
	}
	q, err := client.Question(ctx, slug)
	if err != nil {
		return err
	}
	// LeetCode accepts slugs in any case; compare with its canonical one.
	existing, err := a.WS.Problem(q.TitleSlug)
	if err != nil || existing.Slug != q.TitleSlug {
		existing = nil
	}
	if existing != nil && !force {
		return fmt.Errorf("already in the bank at %s (use --force to replace)", existing.Dir)
	}
	conv, err := leetcode.Convert(q, client.Site, translated)
	if err != nil {
		return err
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
		conv.Problem.ID, conv.Problem.Title, dir, len(conv.Cases), withExpected)
	for _, w := range conv.Warnings {
		fmt.Printf("    %s %s\n", ui.Yellow.Render("note:"), w)
	}
	fmt.Printf("    next: hc gen %s — writes the test cases and decides how answers are judged\n", conv.Problem.Slug)
	return nil
}
