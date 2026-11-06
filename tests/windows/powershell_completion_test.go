package windows_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"
)

func completionProfilePaths(c cli) []string {
	return []string{filepath.Join(c.root, "PowerShell", "profile.ps1"), filepath.Join(c.root, "WindowsPowerShell", "profile.ps1")}
}

func TestPowerShellCompletionActivation(t *testing.T) {
	c := newCLI(t)
	setCLIEnv(&c, "SKOPEO_POWERSHELL_COMPLETION", "on")
	profiles := completionProfilePaths(c)
	original := []byte("# unrelated user settings\r\n$global:PreservedSetting = 'unchanged'\r\n")
	write(t, profiles[0], original)
	units := utf16.Encode([]rune("# existing UTF-16 settings\r\n$global:UnicodeSetting = 'ok'\r\n"))
	wide := []byte{0xff, 0xfe}
	for _, u := range units {
		wide = binary.LittleEndian.AppendUint16(wide, u)
	}
	write(t, profiles[1], wide)
	setup := c.executeExact(t, "", nil, "completion")
	if !bytes.Contains(setup.stdout, []byte("Restart PowerShell")) {
		t.Fatal("setup did not prompt a PowerShell restart")
	}
	first := read(t, profiles[0])
	second := read(t, profiles[1])
	if !bytes.HasPrefix(first, original) || !bytes.HasPrefix(second, wide) || !bytes.Contains(first, []byte("skopeo-win PowerShell completion")) {
		t.Fatal("explicit setup lost profile contents or encoding")
	}
	c.executeExact(t, "", nil, "completion")
	if !bytes.Equal(first, read(t, profiles[0])) || !bytes.Equal(second, read(t, profiles[1])) {
		t.Fatal("explicit setup is not idempotent")
	}
	status := c.executeExact(t, "", nil, "completion", "--status")
	if !bytes.Contains(status.stdout, []byte("PowerShell profile activation disabled: false")) {
		t.Fatal("enabled status is incorrect")
	}
	c.executeExact(t, "", nil, "completion", "--uninstall")
	if !bytes.Equal(original, read(t, profiles[0])) || !bytes.Equal(wide, read(t, profiles[1])) {
		t.Fatal("uninstall changed unrelated profile bytes")
	}
	c.executeExact(t, "", nil)
	if !bytes.Equal(original, read(t, profiles[0])) {
		t.Fatal("disabled completion was automatically re-enabled")
	}
	status = c.executeExact(t, "", nil, "completion", "--status")
	if !bytes.Contains(status.stdout, []byte("PowerShell profile activation disabled: true")) {
		t.Fatal("disabled status is incorrect")
	}
	c.executeExact(t, "", nil, "completion")
	if !bytes.Contains(read(t, profiles[0]), []byte("skopeo-win PowerShell completion")) {
		t.Fatal("explicit re-enable failed")
	}
	assertNoDefaultPolicy(t, c.root)
	bad := c.executeExact(t, "absolute", nil, "completion", "--profile", "relative.ps1")
	if bad.exitCode != 1 {
		t.Fatal("relative profile accepted")
	}
}

func TestPowerShellCompletionCreatesOnlySelectedProfile(t *testing.T) {
	c := newCLI(t)
	profile := filepath.Join(c.root, "custom host", "Microsoft.VSCode_profile.ps1")
	c.executeExact(t, "", nil, "completion", "--profile", profile)
	first := read(t, profile)
	if !bytes.Contains(first, []byte("skopeo-win PowerShell completion")) {
		t.Fatal("missing profile was not created")
	}
	c.executeExact(t, "", nil, "completion", "--profile", profile)
	if !bytes.Equal(first, read(t, profile)) {
		t.Fatal("selected profile setup is not idempotent")
	}
	for _, other := range completionProfilePaths(c) {
		if _, err := os.Stat(other); !os.IsNotExist(err) {
			t.Fatal("setup modified an unselected profile")
		}
	}
	assertNoDefaultPolicy(t, c.root)
}

func TestPowerShellCompletionPreservesOpaqueProfileAndRejectsAmbiguity(t *testing.T) {
	c := newCLI(t)
	profiles := completionProfilePaths(c)
	legacy := []byte{'#', ' ', 0x81, 0x82, '\r', '\n'}
	write(t, profiles[0], legacy)
	c.executeExact(t, "", nil, "completion", "powershell", "--install")
	c.executeExact(t, "", nil, "completion", "powershell", "--uninstall")
	if !bytes.Equal(legacy, read(t, profiles[0])) {
		t.Fatal("legacy profile bytes were changed")
	}
	ambiguous := []byte("\r\n# >>> skopeo-win PowerShell completion >>>\r\n# unmatched marker\r\n")
	write(t, profiles[0], ambiguous)
	c.executeExact(t, "ambiguous", nil, "completion", "powershell", "--install")
	if !bytes.Equal(ambiguous, read(t, profiles[0])) {
		t.Fatal("ambiguous profile was overwritten")
	}
	c = newCLI(t)
	profiles = completionProfilePaths(c)
	sentinel := filepath.Join(c.root, "sentinel.ps1")
	write(t, sentinel, []byte("# unrelated sentinel\r\n"))
	if err := os.MkdirAll(filepath.Dir(profiles[0]), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(sentinel, profiles[0]); err != nil {
		t.Fatal(err)
	}
	c.executeExact(t, "hard-linked", nil, "completion", "powershell", "--install")
	if string(read(t, sentinel)) != "# unrelated sentinel\r\n" {
		t.Fatal("a hard-linked user file was modified")
	}
	c = newCLI(t)
	profiles = completionProfilePaths(c)
	signed := []byte("# user profile\r\n# SIG # Begin signature block\r\n# signature\r\n# SIG # End signature block\r\n")
	write(t, profiles[0], signed)
	c.executeExact(t, "signed PowerShell profile", nil, "completion", "powershell", "--install")
	if !bytes.Equal(signed, read(t, profiles[0])) {
		t.Fatal("a signed profile was modified")
	}
}

func TestPowerShellReadOnlyCommandsDoNotConfigureProfiles(t *testing.T) {
	c := newCLI(t)
	setCLIEnv(&c, "SKOPEO_POWERSHELL_COMPLETION", "on")
	manifest := filepath.Join(c.root, "manifest.json")
	write(t, manifest, []byte(`{"schemaVersion":2}`))
	for _, args := range [][]string{nil, {"manifest-digest", manifest}, {"--version"}, {"completion", "--status"}, {"completion", "powershell"}, {"__complete", "inspect", "--override-os", "li"}} {
		c.executeExact(t, "", nil, args...)
	}
	c.executeExact(t, "must not be negative", nil, "--command-timeout=-1s")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected registry request", 500)
	}))
	defer server.Close()
	c.executeExact(t, "", nil, "__complete", "inspect", "docker://"+strings.TrimPrefix(server.URL, "http://")+"/image")
	if requests.Load() != 0 {
		t.Fatal("completion contacted a registry")
	}
	assertNoDefaultPolicy(t, c.root)
	for _, path := range completionProfilePaths(c) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("ordinary, read-only or failed command configured a profile")
		}
	}
}

func TestPowerShellCompletionHelp(t *testing.T) {
	c := newCLI(t)
	setCLIEnv(&c, "SKOPEO_POWERSHELL_COMPLETION", "on")
	root := c.executeExact(t, "", nil, "--help")
	listed := false
	for line := range strings.SplitSeq(string(root.stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "completion" && strings.Contains(line, "PowerShell") {
			listed = true
		}
	}
	if !listed || bytes.Contains(root.stdout, []byte("autocomplete")) {
		t.Fatal("root help must advertise completion for PowerShell without autocomplete")
	}
	help := c.executeExact(t, "", nil, "completion", "--help")
	for _, want := range []string{"Configure PowerShell 5.1/7 Tab completion", "Restart PowerShell", "--status", "--uninstall", "--profile"} {
		if !bytes.Contains(help.stdout, []byte(want)) {
			t.Fatalf("completion help is missing %q", want)
		}
	}
	removed := c.executeExact(t, "autocomplete", nil, "autocomplete")
	if removed.exitCode != 1 {
		t.Fatal("removed autocomplete command must fail")
	}
	for _, profile := range completionProfilePaths(c) {
		if _, err := os.Stat(profile); !os.IsNotExist(err) {
			t.Fatal("help or removed command configured a profile")
		}
	}
	assertNoDefaultPolicy(t, c.root)
}

func TestPowerShellTabExpansion(t *testing.T) {
	for _, shell := range []string{"pwsh.exe", "powershell.exe"} {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Fatal(err)
			}
			c := newCLI(t)
			// Windows PowerShell 5.1 leaves $PROFILE unset when Documents is absent
			// at startup. Create only the sandbox Documents directory for discovery.
			if err := os.MkdirAll(filepath.Join(c.root, "Documents"), 0700); err != nil {
				t.Fatal(err)
			}
			// Quotes, spaces and metacharacters in executable paths remain literal.
			binary := filepath.Join(c.root, "\u5de5\u5177 tool space ' $; [] folder", "skopeo-win.exe")
			write(t, binary, read(t, executable))
			completion := filepath.Join(c.root, "completion.ps1")
			r := c.executeExact(t, "", nil, "completion", "powershell")
			if bytes.Contains(r.stdout, []byte("Invoke-Expression")) {
				t.Fatal("completion evaluates command text")
			}
			write(t, completion, r.stdout)
			for _, profile := range completionProfilePaths(c) {
				if _, err := os.Stat(profile); !os.IsNotExist(err) {
					t.Fatal("script generation wrote a profile")
				}
			}
			profileIndex := 0
			if shell == "powershell.exe" {
				profileIndex = 1
			}
			profile := completionProfilePaths(c)[profileIndex]
			lines := []struct{ line, want string }{
				{"skopeo-win ins", "inspect"},
				{"skopeo copy --dest-t", "--dest-tls-verify"},
				{"skopeo-win inspect --override-os li", "linux"},
				{"skopeo-win inspect --override-arch=arm", "--override-arch=arm64"},
				{"skopeo-win copy --multi-arch in", "index-only"},
				{"skopeo-win copy --multi-arch=linux/amd64,wi", "windows/arm64"},
				{"skopeo copy -f o", "oci"},
				{"skopeo-win sync --src y", "yaml"},
				{"skopeo-win copy doc", "docker:"},
				{"& '" + strings.ReplaceAll(binary, "'", "''") + "' ins", "inspect"},
				{"skopeo-win --tmpdir 'a; $b $(throw ''literal'')' ins", "inspect"},
			}
			payload := []string{}
			for _, tc := range lines {
				payload = append(payload, tc.line)
			}
			encoded, _ := json.Marshal(payload)
			check := `$ErrorActionPreference='Stop'
# Default discovery is read-only here; all setup below uses the sandbox profile.
$savedProfiles = $env:SKOPEO_POWERSHELL_PROFILES
try {
    $env:SKOPEO_POWERSHELL_PROFILES = $null
    $status = & $env:COMPLETION_BINARY completion --status
    if ($LASTEXITCODE -ne 0) { throw 'Default profile discovery failed' }
    if (-not (($status -join [Environment]::NewLine).Contains('Profile: ' + $PROFILE.CurrentUserCurrentHost + ';'))) {
        throw ('Default profile does not match the calling console host: expected ' + $PROFILE.CurrentUserCurrentHost + '; received ' + ($status -join ' '))
    }
} finally { $env:SKOPEO_POWERSHELL_PROFILES = $savedProfiles }
$setup = & $env:COMPLETION_BINARY completion --profile $env:COMPLETION_PROFILE
if ($LASTEXITCODE -ne 0 -or -not (($setup -join ' ').Contains('Restart PowerShell'))) { throw 'Explicit setup failed' }
$profileBefore = [Convert]::ToBase64String([IO.File]::ReadAllBytes($env:COMPLETION_PROFILE))
. $env:COMPLETION_PROFILE
$lines = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($env:COMPLETION_LINES)) | ConvertFrom-Json
$results = @()
foreach ($line in $lines) {
    try { $matches = [Management.Automation.CommandCompletion]::CompleteInput($line, $line.Length, $null).CompletionMatches }
    catch { throw ('Completion line failed: ' + $line + ' / ' + $_.Exception.Message) }
    $results += ,@($matches | ForEach-Object { $_.CompletionText.Trim() })
}
$sentinel = Join-Path $env:COMPLETION_ROOT 'injected.txt'
$malicious = 'skopeo-win --tmpdir "$(Set-Content -LiteralPath ''' + $sentinel.Replace("'","''") + ''' owned)" ins'
$null = [Management.Automation.CommandCompletion]::CompleteInput($malicious, $malicious.Length, $null)
if (Test-Path -LiteralPath $sentinel) { throw 'Completion executed an argument expression' }
if ($profileBefore -ne [Convert]::ToBase64String([IO.File]::ReadAllBytes($env:COMPLETION_PROFILE))) { throw 'Tab completion changed the profile' }
ConvertTo-Json -InputObject @($results) -Compress -Depth 10
`
			script := filepath.Join(c.root, "check.ps1")
			write(t, script, []byte(check))
			setCLIEnv(&c, "COMPLETION_PROFILE", profile)
			setCLIEnv(&c, "COMPLETION_BINARY", executable)
			setCLIEnv(&c, "SKOPEO_POWERSHELL_COMPLETION", "on")
			setCLIEnv(&c, "COMPLETION_LINES", base64.StdEncoding.EncodeToString(encoded))
			setCLIEnv(&c, "COMPLETION_ROOT", c.root)
			// A semicolon is valid in a quoted executable path but is a PATH separator.
			// Keep ordinary aliases in a separate directory and test the literal path too.
			aliasDirectory := filepath.Join(c.root, "aliases")
			setCLIEnv(&c, "PATH", aliasDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
			write(t, filepath.Join(aliasDirectory, "skopeo.exe"), read(t, executable))
			write(t, filepath.Join(aliasDirectory, "skopeo-win.exe"), read(t, executable))
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, path, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script)
			cmd.Env = c.env
			cmd.Dir = c.root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("PowerShell completion failed: %v\n%s", err, out)
			}
			var results [][]string
			if err := json.Unmarshal(bytes.TrimPrefix(out, []byte{0xef, 0xbb, 0xbf}), &results); err != nil {
				t.Fatalf("invalid shell results: %v / %s", err, out)
			}
			if len(results) != len(lines) {
				t.Fatalf("wrong result count: %s", out)
			}
			for i, tc := range lines {
				found := false
				for _, value := range results[i] {
					if value == tc.want {
						found = true
					}
				}
				if !found {
					t.Errorf("%s did not complete %q: %v", tc.line, tc.want, results[i])
				}
			}
			assertNoDefaultPolicy(t, c.root)
			if !bytes.Contains(read(t, profile), []byte("skopeo-win PowerShell completion")) {
				t.Fatal("explicit setup did not create the shell's profile")
			}
			if _, err := os.Stat(completionProfilePaths(c)[1-profileIndex]); !os.IsNotExist(err) {
				t.Fatal("setup changed the other shell's profile")
			}
		})
	}
}
