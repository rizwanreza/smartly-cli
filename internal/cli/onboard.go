package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/rizwanreza/smartly-cli/internal/config"
	"github.com/rizwanreza/smartly-cli/internal/onboard"
)

var onboardDryRunFlag bool

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Set smartly up, one question at a time",
	Long: `Walk through smartly's settings interactively and write a config file.

Nothing is written until you confirm, an existing config is backed up
first, and smartly never asks for an API key — it looks for the
environment variable and tells you the export line if it isn't set.

For a non-interactive setup, use ` + "`smartly config init`" + ` instead.`,
	Args:          cobra.NoArgs,
	RunE:          runOnboard,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	onboardCmd.Flags().BoolVar(&onboardDryRunFlag, "dry-run", false, "walk through the questions and print the config, without writing it")
	rootCmd.AddCommand(onboardCmd)
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// Fail closed with no controlling terminal, the same way the confirm
	// gate does, and point at the command that does work without one.
	tty, err := openTTY()
	if err != nil {
		return fmt.Errorf("smartly onboard needs an interactive terminal, and there isn't one here; run `smartly config init` to write a default config file instead")
	}
	defer tty.Close()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	configPath := config.Path()
	_, statErr := os.Stat(configPath)
	configExists := statErr == nil

	answers, err := askOnboardingQuestions(tty, cfg, configPath, configExists)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(tty, "\n"+mutedStyle.Render("Stopped. Nothing was written."))
			return nil
		}
		return err
	}

	result, err := onboard.Apply(cfg, *answers)
	if err != nil {
		return err
	}

	return reviewAndWrite(tty, result, *answers, configPath, configExists)
}

// askOnboardingQuestions runs the interactive flow. It returns the
// collected answers; every decision it makes about *which* questions to
// ask mirrors internal/onboard.Questions, which is where that branching is
// tested.
func askOnboardingQuestions(tty io.ReadWriter, cfg *config.Config, configPath string, configExists bool) (*onboard.Answers, error) {
	fmt.Fprint(tty, welcomeHeader())

	if configExists {
		fmt.Fprint(tty, "\n"+foundStyle.Render("✓ found your existing config")+" "+mutedStyle.Render(configPath)+"\n")
		fmt.Fprint(tty, mutedStyle.Render("  Your answers start from what's in it. It gets backed up before anything is replaced.")+"\n")
	}

	a := onboard.SeedFromConfig(cfg)
	a.Shell = onboard.DefaultDetector().DetectShell()

	if err := askProvider(tty, cfg, &a); err != nil {
		return nil, err
	}
	if err := askModel(tty, cfg, &a); err != nil {
		return nil, err
	}
	if err := askExecutionMode(tty, &a); err != nil {
		return nil, err
	}
	if err := askContext(tty, &a); err != nil {
		return nil, err
	}
	if err := askLogPath(tty, &a); err != nil {
		return nil, err
	}
	if err := askShell(tty, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func askProvider(tty io.ReadWriter, cfg *config.Config, a *onboard.Answers) error {
	statuses := onboard.DefaultDetector().Detect(cfg)
	byName := map[string]onboard.ProviderStatus{}

	width := 0
	for _, st := range statuses {
		byName[st.Name] = st
		if len(st.Name) > width {
			width = len(st.Name)
		}
	}

	options := make([]huh.Option[string], 0, len(statuses))
	for _, st := range statuses {
		label := fmt.Sprintf("%-*s  %s", width, st.Name, annotate(st.Detail, st.Ready))
		options = append(options, huh.NewOption(label, st.Name))
	}

	fmt.Fprint(tty, stepHeading("Which model runs your sentences?"))

	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider").
			Options(options...).
			Value(&a.Provider).
			DescriptionFunc(func() string {
				st := byName[a.Provider]
				desc := st.Summary
				if !st.Ready && st.Fix != "" {
					desc += "\nnot ready yet — " + st.Fix
				}
				return desc
			}, &a.Provider),
	))
	if err := form.Run(); err != nil {
		return err
	}

	// Selecting a different provider re-seeds the model from whatever that
	// provider already had configured, rather than carrying the previous
	// provider's model across.
	a.Model = onboard.ModelFor(cfg, a.Provider)

	if st := byName[a.Provider]; !st.Ready && st.Fix != "" {
		fmt.Fprint(tty, "\n"+cautionStyle.Render("! "+a.Provider+" isn't ready on this machine yet")+"\n")
		fmt.Fprint(tty, mutedStyle.Render("  "+st.Fix)+"\n")
		fmt.Fprint(tty, mutedStyle.Render("  Carry on — this config will be waiting when you are.")+"\n")
	}
	return nil
}

func askModel(tty io.ReadWriter, cfg *config.Config, a *onboard.Answers) error {
	required := onboard.RequiresModel(a.Provider)
	if !required && a.Provider != "anthropic" {
		return nil
	}

	title := "Model"
	desc := "Leave it as-is unless you have a reason."
	if required {
		desc = "Required: smartly ships no default for " + a.Provider + "."
	}

	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewInput().
			Title(title).
			Description(desc).
			Placeholder(onboard.ModelFor(config.Defaults(), a.Provider)).
			Value(&a.Model).
			Validate(func(s string) error {
				if required && strings.TrimSpace(s) == "" {
					return fmt.Errorf("%s needs a model name", a.Provider)
				}
				return nil
			}),
	))
	if err := form.Run(); err != nil {
		return err
	}
	a.Model = strings.TrimSpace(a.Model)
	return nil
}

func askExecutionMode(tty io.ReadWriter, a *onboard.Answers) error {
	fmt.Fprint(tty, stepHeading("How careful should smartly be?"))

	modes := onboard.ModeOptions()
	descriptions := map[string]string{}
	options := make([]huh.Option[string], 0, len(modes))
	for _, m := range modes {
		descriptions[m.Value] = m.Description
		options = append(options, huh.NewOption(m.Title, m.Value))
	}
	descriptions[config.ModeConfirmDestructive] += "\n" + onboard.ClassifierHonesty()

	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewNote().
			Title("This is a default, not a cage").
			Description(onboard.ModeEducation()),
		huh.NewSelect[string]().
			Title("Execution mode").
			Options(options...).
			Value(&a.Mode).
			DescriptionFunc(func() string { return descriptions[a.Mode] }, &a.Mode),
	))
	if err := form.Run(); err != nil {
		return err
	}

	if a.Mode != config.ModeConfirmDestructive {
		return nil
	}
	return offerClassifierDemo(tty)
}

// offerClassifierDemo runs the real classifier over a handful of commands
// in front of the user. It is the honest way to explain a heuristic: show
// it working, including the case where it shrugs.
func offerClassifierDemo(tty io.ReadWriter) error {
	// Defaults to yes: the demo is the whole argument for this mode, and
	// showing it costs nothing.
	wantDemo := true
	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewConfirm().
			Title("Want to see it decide?").
			Description("Four commands, real verdicts, nothing runs.").
			Affirmative("Show me").
			Negative("Skip").
			Value(&wantDemo),
	))
	if err := form.Run(); err != nil {
		return err
	}
	if !wantDemo {
		return nil
	}

	fmt.Fprint(tty, "\n"+renderClassifierDemo(onboard.ClassifierDemoCommands()))
	fmt.Fprint(tty, mutedStyle.Render("  The last one isn't dangerous — it's just unrecognized, and that's enough to ask.")+"\n")
	return nil
}

func askContext(tty io.ReadWriter, a *onboard.Answers) error {
	fmt.Fprint(tty, stepHeading("How much can smartly see?"))

	levels := onboard.ContextOptions()
	descriptions := map[string]string{}
	options := make([]huh.Option[string], 0, len(levels))
	for _, l := range levels {
		descriptions[l.Value] = l.Description
		options = append(options, huh.NewOption(l.Title, l.Value))
	}

	previous := a.Context
	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewSelect[string]().
			Title("Context level").
			Options(options...).
			Value(&a.Context).
			DescriptionFunc(func() string { return descriptions[a.Context] }, &a.Context),
	))
	if err := form.Run(); err != nil {
		return err
	}

	if a.Context != "full" {
		a.FullContextConfirmed = false
		return nil
	}
	// Choosing full where it wasn't already chosen requires a second,
	// explicit confirmation with the consequence stated in full. A single
	// keypress must not be able to opt someone into shipping their shell
	// history to a third party.
	if previous == "full" && a.FullContextConfirmed {
		return nil
	}

	a.FullContextConfirmed = false
	confirmForm := newOnboardForm(tty, huh.NewGroup(
		huh.NewNote().
			Title("Before you pick full").
			Description(onboard.FullContextWarning()),
		huh.NewConfirm().
			Title("Send your shell history with every request?").
			Affirmative("Yes, I've read that").
			Negative("No, use light").
			Value(&a.FullContextConfirmed),
	))
	if err := confirmForm.Run(); err != nil {
		return err
	}
	if !a.FullContextConfirmed {
		a.Context = "light"
		fmt.Fprint(tty, mutedStyle.Render("  Kept at light. Good call.")+"\n")
	}
	return nil
}

func askLogPath(tty io.ReadWriter, a *onboard.Answers) error {
	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewInput().
			Title("History log").
			Description("Append-only record of every request. Stores your sentences and commands verbatim, at 0600.").
			Value(&a.LogPath).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("a log path is required")
				}
				return nil
			}),
	))
	if err := form.Run(); err != nil {
		return err
	}
	a.LogPath = strings.TrimSpace(a.LogPath)
	return nil
}

func askShell(tty io.ReadWriter, a *onboard.Answers) error {
	choices := onboard.ShellChoices()
	options := make([]huh.Option[string], 0, len(choices))
	for _, c := range choices {
		options = append(options, huh.NewOption(c.Title, c.Value))
	}

	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewSelect[string]().
			Title("Shell integration").
			Description("Lets a generated cd or export actually affect your shell.\nsmartly prints a line for you to add — it never edits your rc file.").
			Options(options...).
			Value(&a.Shell),
	))
	return form.Run()
}

// reviewAndWrite shows the resolved config, asks once, and only then
// touches the disk.
func reviewAndWrite(tty io.ReadWriter, result *config.Config, a onboard.Answers, configPath string, configExists bool) error {
	fmt.Fprint(tty, stepHeading("Here's what that adds up to"))
	printResolvedConfig(tty, result)

	if onboardDryRunFlag {
		fmt.Fprint(tty, stepHeading("This is the file it would write"))
		fmt.Fprint(tty, renderConfigForDisplay(result))
		fmt.Fprint(tty, shellReminder(a))
		fmt.Fprint(tty, "\n"+mutedStyle.Render("--dry-run: nothing was written.")+"\n")
		return nil
	}

	var write bool
	form := newOnboardForm(tty, huh.NewGroup(
		huh.NewConfirm().
			Title("Write this to "+configPath+"?").
			DescriptionFunc(func() string {
				if configExists {
					return "Your current config gets copied to a timestamped backup first."
				}
				return "This creates the file. Nothing else on your machine changes."
			}, nil).
			Affirmative("Write it").
			Negative("Not now").
			Value(&write),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			write = false
		} else {
			return err
		}
	}

	if !write {
		fmt.Fprint(tty, stepHeading("Nothing was written"))
		fmt.Fprint(tty, mutedStyle.Render("Here it is anyway, if you'd rather write it yourself — "+configPath)+"\n\n")
		fmt.Fprint(tty, renderConfigForDisplay(result))
		return nil
	}

	backup, err := onboard.BackupExisting(configPath, time.Now())
	if err != nil {
		return err
	}
	if err := writeConfigFile(configPath, renderConfigTemplate(result)); err != nil {
		return err
	}

	fmt.Fprint(tty, "\n")
	if backup != "" {
		fmt.Fprint(tty, foundStyle.Render("✓ backed up ")+mutedStyle.Render(backup)+"\n")
	}
	fmt.Fprint(tty, foundStyle.Render("✓ wrote ")+configPath+"\n")
	fmt.Fprint(tty, sendOff(a))
	return nil
}

func writeConfigFile(path, contents string) error {
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// sendOff is the last thing on screen: what to do next, in the smallest
// number of words that will actually get someone to try it.
func sendOff(a onboard.Answers) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(headingStyle.Render("Try it"))
	b.WriteString("\n")
	for _, s := range onboard.TrySuggestions() {
		b.WriteString("  " + tryStyle.Render(s) + "\n")
	}
	b.WriteString(shellReminder(a))
	return b.String()
}

// shellReminder prints the line the user adds to their own rc file.
// smartly prints it and stops there — it never edits an rc file, because
// editing someone's shell startup without being asked is not a favor.
func shellReminder(a onboard.Answers) string {
	if a.Shell == "" {
		return ""
	}
	return "\n" +
		mutedStyle.Render("One more thing — add this to "+onboard.RCFile(a.Shell)+" yourself, then open a new shell:") +
		"\n  " + tryStyle.Render(onboard.EvalLine(a.Shell)) + "\n"
}

// renderConfigForDisplay renders the config file with any pre-existing
// api_key replaced by a note. Onboarding never collects a key, but a key
// already in the user's file is carried through on write — and printing a
// file to a terminal is not the same as writing it to a 0600 file, so the
// displayed copy never contains the value.
func renderConfigForDisplay(cfg *config.Config) string {
	shown := *cfg
	redacted := false
	if shown.Providers.Anthropic.APIKey != "" {
		shown.Providers.Anthropic.APIKey = keptKeyPlaceholder
		redacted = true
	}
	if shown.Providers.OpenAI.APIKey != "" {
		shown.Providers.OpenAI.APIKey = keptKeyPlaceholder
		redacted = true
	}

	out := renderConfigTemplate(&shown)
	if redacted {
		out += "\n" + mutedStyle.Render("# The api_key already in your config is kept as-is, and not shown here.") + "\n"
	}
	return out
}

const keptKeyPlaceholder = "<kept from your existing config, not shown>"
