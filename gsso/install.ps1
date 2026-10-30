# Installs or updates gsso to the latest release in %LOCALAPPDATA%\Programs\gsso and registers the PowerShell hook.
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
  $exe = Join-Path $dir 'gsso.exe'
  $old = if (Test-Path $exe) { ((& $exe --version) -split ' ')[-1] } else { '' }

  # Download beside the binary and move it over, so a failed download leaves the old binary intact.
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $tmp = Join-Path $dir 'gsso.exe.download'
  Invoke-WebRequest -UseBasicParsing $asset.browser_download_url -OutFile $tmp
  Move-Item -Force -Path $tmp -Destination $exe
  $new = ((& $exe --version) -split ' ')[-1]

  $found = (Get-Command gsso.exe -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1).Source
  if ($found -and $found -ne $exe) {
    Write-Warning "$found comes before $exe on PATH, so this session still runs that one."
  }

  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir".TrimStart(';'), 'User')
  }

  if (-not (Test-Path $PROFILE)) { New-Item -ItemType File -Force -Path $PROFILE | Out-Null }
  $line = 'gsso shell-init powershell | Out-String | Invoke-Expression'
  if (-not (Select-String -LiteralPath $PROFILE -SimpleMatch $line -Quiet)) {
    Add-Content -LiteralPath $PROFILE -Value $line
  }

  if (-not $old) {
    Write-Host "Installed gsso $new to $dir. Open a new terminal."
  } elseif ($old -eq $new) {
    Write-Host "gsso $new is already the latest release."
  } else {
    Write-Host "Updated gsso $old -> $new. Open a new terminal to load the updated hook."
  }
}
