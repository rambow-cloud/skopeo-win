# skopeo-win PowerShell completion (managed by skopeo-win)
# Native process arguments are quoted for Windows, never evaluated as script.
$skopeoCommandNames = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('@@COMMAND_NAMES@@')) | ConvertFrom-Json
Register-ArgumentCompleter -Native -CommandName $skopeoCommandNames -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    try {
        $elements = @($commandAst.CommandElements | Where-Object { $_.Extent.StartOffset -lt $cursorPosition })
        if ($elements.Count -eq 0) { return }
        $values = [Collections.Generic.List[string]]::new()
        foreach ($element in $elements) {
            if ($element.Extent.EndOffset -gt $cursorPosition) { return }
            if ($element -is [Management.Automation.Language.StringConstantExpressionAst] -or
                $element -is [Management.Automation.Language.ConstantExpressionAst]) {
                $values.Add([string]$element.Value)
            } elseif ($element -is [Management.Automation.Language.ExpandableStringExpressionAst] -and $element.NestedExpressions.Count -eq 0) {
                $values.Add([string]$element.Value)
            } elseif ($element -is [Management.Automation.Language.CommandParameterAst] -and $null -eq $element.Argument) {
                $values.Add($element.Extent.Text)
            } elseif ($element -is [Management.Automation.Language.ArrayLiteralAst]) {
                $parts = [Collections.Generic.List[string]]::new()
                foreach ($part in $element.Elements) {
                    if ($part -is [Management.Automation.Language.StringConstantExpressionAst] -or $part -is [Management.Automation.Language.ConstantExpressionAst]) { $parts.Add([string]$part.Value) }
                    elseif ($part -is [Management.Automation.Language.ExpandableStringExpressionAst] -and $part.NestedExpressions.Count -eq 0) { $parts.Add([string]$part.Value) }
                    else { return }
                }
                $values.Add($parts -join ',')
            } else {
                # Do not expand variables, subexpressions, pipelines or script blocks.
                return
            }
        }
        if ($values[0] -match '[\\/]') {
            $executablePath = $values[0]
            if (-not [IO.Path]::IsPathRooted($executablePath)) { $executablePath = Join-Path $ExecutionContext.SessionState.Path.CurrentFileSystemLocation.Path $executablePath }
            if (-not (Test-Path -LiteralPath $executablePath -PathType Leaf)) { return }
        } else {
            $application = Get-Command -Name $values[0] -CommandType Application -ErrorAction SilentlyContinue |
                Where-Object { $_.Name -ieq $values[0] -or [IO.Path]::GetFileNameWithoutExtension($_.Name) -ieq $values[0] } | Select-Object -First 1
            if (-not $application) { return }
            $executablePath = $application.Source
        }
        $arguments = [Collections.Generic.List[string]]::new()
        $arguments.Add('@@REQUEST@@')
        for ($i = 1; $i -lt $values.Count; $i++) { $arguments.Add($values[$i]) }
        if ($elements[-1].Extent.EndOffset -lt $cursorPosition) { $arguments.Add('') }
        if ($arguments.Count -lt 2) { $arguments.Add('') }
        $quoted = foreach ($argument in $arguments) {
            '"' + (($argument -replace '(\\*)"', '$1$1\"') -replace '(\\+)$', '$1$1') + '"'
        }
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = $executablePath
        $start.WorkingDirectory = $ExecutionContext.SessionState.Path.CurrentFileSystemLocation.Path
        $start.Arguments = $quoted -join ' '
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
        $start.RedirectStandardOutput = $true
        $start.RedirectStandardError = $true
        $process = [Diagnostics.Process]::Start($start)
        try {
            $stdout = $process.StandardOutput.ReadToEndAsync()
            $stderr = $process.StandardError.ReadToEndAsync()
            if (-not $process.WaitForExit(5000)) { $process.Kill(); return }
            if ($process.ExitCode -ne 0) { return }
            $lines = @($stdout.GetAwaiter().GetResult() -split '\r?\n' | Where-Object { $_ -ne '' })
            if ($lines.Count -eq 0 -or $lines[-1] -notmatch '^:([0-9]+)$') { return }
            $directive = [int]$Matches[1]
            if (($directive -band 1) -ne 0) { return }
            $entries = @($lines | Select-Object -SkipLast 1)
            if ($entries.Count -eq 0) { return }
            foreach ($entry in $entries) {
                $parts = $entry -split "`t", 2
                $name = $parts[0]
                if ($name.StartsWith('_activeHelp_')) { continue }
                $description = if ($parts.Count -gt 1 -and $parts[1]) { $parts[1] } else { $name }
                if ($wordToComplete -like '--*=*' -and -not $name.StartsWith('--')) {
                    $name = ($wordToComplete -split '=', 2)[0] + '=' + $name
                }
                if ($elements[-1] -is [Management.Automation.Language.ArrayLiteralAst] -and $wordToComplete -notlike '*,*') {
                    # PowerShell replaces only the current comma-separated element.
                    $name = ($name -split ',')[-1]
                }
                $text = $name
                if ($name -match '[\s;|&$`"''(){}<>]') { $text = "'" + $name.Replace("'", "''") + "'" }
                if (($directive -band 2) -eq 0) { $text += ' ' }
                [Management.Automation.CompletionResult]::new($text, $name, 'ParameterValue', $description)
            }
        } finally { $process.Dispose() }
    } catch { return }
}
