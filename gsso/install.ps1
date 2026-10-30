# Installs the latest gsso release to %LOCALAPPDATA%\Programs\gsso and registers the PowerShell hook.
#   irm https://raw.githubusercontent.com/twentyfourg/24g-aws-sso/main/gsso/install.ps1 | iex
# Runs in a script block so the preference changes and any error stay out of the caller's session.
& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'

  $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
  $asset = (Invoke-RestMethod 'https://api.github.com/repos/twentyfourg/24g-aws-sso/releases?per_page=100').assets |
    Where-Object name -eq "gsso-windows-$arch.exe" | Select-Object -First 1
  if (-not $asset) { throw "No gsso release found for windows-$arch" }

  $dir = Join-Path $env:LOCALAPPDATA 'Programs\gsso'
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Invoke-WebRequest -UseBasicParsing $asset.browser_download_url -OutFile (Join-Path $dir 'gsso.exe')

  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir".TrimStart(';'), 'User')
  }

  if (-not (Test-Path $PROFILE)) { New-Item -ItemType File -Force -Path $PROFILE | Out-Null }
  $line = 'gsso shell-init powershell | Out-String | Invoke-Expression'
  if (-not (Select-String -LiteralPath $PROFILE -SimpleMatch $line -Quiet)) {
    Add-Content -LiteralPath $PROFILE -Value $line
  }

  Write-Host "Installed $(& (Join-Path $dir 'gsso.exe') --version) to $dir. Open a new terminal."
}
