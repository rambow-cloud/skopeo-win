package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

//go:embed completion_powershell.ps1
var powerShellCompletion string

const completionStart = "\r\n# >>> skopeo-win PowerShell completion >>>\r\n"
const completionEnd = "# <<< skopeo-win PowerShell completion <<<\r\n"

func powerShellScript(noDescriptions bool) ([]byte, error) {
	names := []string{"skopeo", "skopeo.exe", "skopeo-win", "skopeo-win.exe", "skopeo-windows-amd64.exe", "skopeo-windows-arm64.exe"}
	if executable, err := os.Executable(); err == nil {
		name := filepath.Base(executable)
		names = append(names, name, strings.TrimSuffix(name, filepath.Ext(name)))
	}
	slices.Sort(names)
	encoded, err := json.Marshal(slices.Compact(names))
	if err != nil {
		return nil, err
	}
	script := strings.ReplaceAll(powerShellCompletion, "@@COMMAND_NAMES@@", base64.StdEncoding.EncodeToString(encoded))
	request := cobra.ShellCompRequestCmd
	if noDescriptions {
		request = cobra.ShellCompNoDescRequestCmd
	}
	return []byte(strings.ReplaceAll(script, "@@REQUEST@@", request)), nil
}

func completionCommand(root *cobra.Command) *cobra.Command {
	parent := &cobra.Command{Use: "completion", Short: "Generate shell completion and manage PowerShell activation", Args: cobra.NoArgs}
	var install, uninstall, status, noDescriptions, showPath bool
	var profiles []string
	ps := &cobra.Command{
		Use: "powershell", Short: "Generate or automatically configure PowerShell completion", Args: cobra.NoArgs,
		Long: "Generate a safe PowerShell 5.1/7 completion script. --install configures user profiles; --uninstall disables automatic activation. New sessions load the profile; a child executable cannot modify an existing parent shell.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showPath {
				if len(profiles) != 0 {
					return errors.New("--profile cannot be combined with --path")
				}
				path, _, err := completionFiles()
				if err == nil {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
				}
				return err
			}
			if !install && !uninstall && !status {
				if len(profiles) != 0 {
					return errors.New("--profile requires --install, --uninstall or --status")
				}
				script, err := powerShellScript(noDescriptions)
				if err == nil {
					_, err = cmd.OutOrStdout().Write(script)
				}
				return err
			}
			selected, err := completionProfiles(profiles)
			if err != nil {
				return err
			}
			if status {
				_, disabled, err := completionFiles()
				if err != nil {
					return err
				}
				_, disabledErr := os.Stat(disabled)
				if disabledErr != nil && !errors.Is(disabledErr, os.ErrNotExist) {
					return disabledErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Automatic activation disabled: %t\n", disabledErr == nil || completionEnvironmentDisabled())
				for _, profile := range selected {
					data, err := os.ReadFile(profile)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s; installed: %t\n", profile, bytes.Contains(data, encodedProfileText(data, completionStart)))
				}
				return nil
			}
			if err := configurePowerShell(selected, uninstall, noDescriptions); err != nil {
				return err
			}
			if uninstall {
				fmt.Fprintln(cmd.OutOrStdout(), "PowerShell completion disabled. Open a new terminal to unload the existing completer.")
			} else {
				path, _, err := completionFiles()
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "PowerShell completion enabled for new sessions. To load it now: . '%s'\n", strings.ReplaceAll(path, "'", "''"))
			}
			return nil
		},
	}
	ps.Flags().BoolVar(&install, "install", false, "configure completion in current-user PowerShell profiles")
	ps.Flags().BoolVar(&uninstall, "uninstall", false, "remove managed profile blocks and disable automatic activation")
	ps.Flags().BoolVar(&status, "status", false, "show automatic activation and profile status")
	ps.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "disable completion descriptions")
	ps.Flags().StringArrayVar(&profiles, "profile", nil, "explicit profile path (repeatable; default: current-user all-hosts profiles)")
	ps.Flags().BoolVar(&showPath, "path", false, "print the installed completion script path for dot-sourcing")
	ps.MarkFlagsMutuallyExclusive("install", "uninstall", "status", "path")
	parent.AddCommand(ps)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var noDescriptions bool
		command := &cobra.Command{Use: shell, Short: "Generate " + shell + " completion", Args: cobra.NoArgs, RunE: func(c *cobra.Command, args []string) error {
			switch shell {
			case "bash":
				return root.GenBashCompletionV2(c.OutOrStdout(), !noDescriptions)
			case "fish":
				return root.GenFishCompletion(c.OutOrStdout(), !noDescriptions)
			default:
				if noDescriptions {
					return root.GenZshCompletionNoDesc(c.OutOrStdout())
				}
				return root.GenZshCompletion(c.OutOrStdout())
			}
		}}
		command.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "disable completion descriptions")
		parent.AddCommand(command)
	}
	return parent
}

func completionFiles() (string, string, error) {
	profiles, err := completionProfiles(nil)
	if err != nil {
		return "", "", err
	}
	// Documents is shared with the host PowerShell, unlike MSIX-virtualized AppData.
	config := filepath.Join(filepath.Dir(profiles[0]), "SkopeoWin")
	return filepath.Join(config, "completion.ps1"), filepath.Join(config, "completion-disabled"), nil
}

func completionProfiles(explicit []string) ([]string, error) {
	if len(explicit) == 0 && os.Getenv("SKOPEO_POWERSHELL_PROFILES") != "" {
		explicit = filepath.SplitList(os.Getenv("SKOPEO_POWERSHELL_PROFILES"))
	}
	if len(explicit) == 0 {
		return defaultPowerShellProfiles()
	}
	for _, path := range explicit {
		if !filepath.IsAbs(path) {
			return nil, errors.New("PowerShell profile paths must be absolute")
		}
	}
	return explicit, nil
}

func encodedProfileText(existing []byte, text string) []byte {
	if len(existing) < 2 || !(bytes.Equal(existing[:2], []byte{0xff, 0xfe}) || bytes.Equal(existing[:2], []byte{0xfe, 0xff})) {
		return []byte(text)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if existing[0] == 0xfe {
		order = binary.BigEndian
	}
	units := utf16.Encode([]rune(text))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		order.PutUint16(data[i*2:], unit)
	}
	return data
}

// editCompletionFile holds a mandatory Windows file lock and keeps a unique
// profile backup. Existing bytes/encoding outside the managed block are retained.
func editCompletionFile(path string, profile bool, edit func([]byte) ([]byte, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(path)
	if info, err := root.Lstat(name); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return errors.New("refusing a linked or non-regular completion file")
	}
	file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	unlock, err := lockCompletionFile(file)
	if err != nil {
		return err
	}
	defer unlock()
	old, err := io.ReadAll(io.LimitReader(file, 4*1024*1024+1))
	if err != nil {
		return err
	}
	if len(old) > 4*1024*1024 {
		return errors.New("completion/profile file is too large")
	}
	if len(old) >= 2 && (old[0] == 0xff || old[0] == 0xfe) && len(old)%2 != 0 {
		return errors.New("invalid UTF-16 profile length")
	}
	next, err := edit(old)
	if err != nil || bytes.Equal(old, next) {
		return err
	}
	if profile && len(old) > 0 {
		backup, err := os.CreateTemp(filepath.Dir(path), name+".skopeo-win-*.bak")
		if err != nil {
			return err
		}
		_, writeErr := backup.Write(old)
		syncErr := backup.Sync()
		closeErr := backup.Close()
		if writeErr != nil {
			return writeErr
		}
		if syncErr != nil {
			return syncErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if _, err = file.WriteAt(next, 0); err == nil {
		err = file.Truncate(int64(len(next)))
	}
	if err == nil {
		err = file.Sync()
	}
	if err != nil {
		_, _ = file.WriteAt(old, 0)
		_ = file.Truncate(int64(len(old)))
		_ = file.Sync()
	}
	return err
}

func configurePowerShell(profiles []string, uninstall, noDescriptions bool) error {
	scriptPath, disabled, err := completionFiles()
	if err != nil {
		return err
	}
	if uninstall {
		if err := editCompletionFile(disabled, false, func(old []byte) ([]byte, error) { return []byte("disabled\n"), nil }); err != nil {
			return err
		}
	} else {
		script, err := powerShellScript(noDescriptions)
		if err != nil {
			return err
		}
		if err := editCompletionFile(scriptPath, false, func(old []byte) ([]byte, error) {
			if len(old) > 0 && !bytes.HasPrefix(old, []byte("# skopeo-win PowerShell completion")) {
				return nil, errors.New("existing completion script is not managed by skopeo-win")
			}
			return script, nil
		}); err != nil {
			return err
		}
		if err := os.Remove(disabled); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(scriptPath))
	block := completionStart + "if ($env:SKOPEO_POWERSHELL_COMPLETION -notin @('off','0','false')) {\r\n" +
		"    $skopeoCompletionPath = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + encoded + "'))\r\n" +
		"    $skopeoCompletionDisabled = Join-Path (Split-Path -Parent $skopeoCompletionPath) 'completion-disabled'\r\n" +
		"    if ((Test-Path -LiteralPath $skopeoCompletionPath) -and -not (Test-Path -LiteralPath $skopeoCompletionDisabled)) { . $skopeoCompletionPath }\r\n}\r\n" + completionEnd
	for _, path := range profiles {
		if uninstall {
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if err := editCompletionFile(path, true, func(old []byte) ([]byte, error) {
			if bytes.Contains(old, encodedProfileText(old, "# SIG # Begin signature block")) {
				if uninstall && !bytes.Contains(old, encodedProfileText(old, completionStart)) {
					return old, nil
				}
				return nil, errors.New("refusing to invalidate a signed PowerShell profile")
			}
			start, end := encodedProfileText(old, completionStart), encodedProfileText(old, completionEnd)
			begin, finish := bytes.Index(old, start), bytes.Index(old, end)
			if bytes.Count(old, start) > 1 || bytes.Count(old, end) > 1 || (begin < 0) != (finish < 0) || begin >= 0 && finish < begin {
				return nil, errors.New("ambiguous managed PowerShell profile block")
			}
			if begin >= 0 {
				old = append(append([]byte{}, old[:begin]...), old[finish+len(end):]...)
			}
			if uninstall {
				return old, nil
			}
			return append(old, encodedProfileText(old, block)...), nil
		}); err != nil {
			return fmt.Errorf("configuring PowerShell profile %s: %w", path, err)
		}
	}
	return nil
}

func completionEnvironmentDisabled() bool {
	value := strings.ToLower(os.Getenv("SKOPEO_POWERSHELL_COMPLETION"))
	return value == "off" || value == "0" || value == "false"
}

func autoConfigurePowerShell(cmd *cobra.Command) {
	if !completionAutoSupported() || cmd == nil {
		return
	}
	if completionEnvironmentDisabled() {
		return
	}
	if os.Getenv("SKOPEO_POWERSHELL_PROFILES") == "" && !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	for parent := cmd; parent != nil; parent = parent.Parent() {
		if parent.Name() == "completion" || parent.Name() == cobra.ShellCompRequestCmd || parent.Name() == cobra.ShellCompNoDescRequestCmd {
			return
		}
	}
	for _, name := range []string{"help", "version"} {
		if flag := cmd.Flags().Lookup(name); flag != nil && flag.Value.String() == "true" {
			return
		}
	}
	_, disabled, err := completionFiles()
	if err != nil {
		return
	}
	if _, err := os.Lstat(disabled); !errors.Is(err, os.ErrNotExist) {
		return
	}
	profiles, err := completionProfiles(nil)
	if err == nil {
		profiles = automaticPowerShellProfiles(profiles)
		if len(profiles) > 0 {
			_ = configurePowerShell(profiles, false, false)
		}
	}
}
