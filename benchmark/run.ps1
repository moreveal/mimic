param([string]$Chrome = "", [switch]$Smoke, [switch]$Resume, [string]$Output = 'benchmark/results')
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
Push-Location $repo
try {
    $python = Join-Path $repo '.build/benchmark-venv/Scripts/python.exe'
    if (-not (Test-Path -LiteralPath $python)) {
        python -m venv .build/benchmark-venv
        if ($LASTEXITCODE -ne 0) { throw 'Cannot create benchmark environment' }
    }
    & $python -m pip install -r benchmark/requirements.txt
    if ($LASTEXITCODE -ne 0) { throw 'Cannot install benchmark dependencies' }
    $arguments = @('benchmark/run.py', '--output', $Output)
    if ($Chrome) { $arguments += @('--chrome', $Chrome) }
    if ($Smoke) { $arguments += @('--smoke', '--output', '.build/benchmark-smoke') }
    if ($Resume) { $arguments += @('--resume', '--skip-build') }
    & $python @arguments
    if ($LASTEXITCODE -ne 0) { throw 'Benchmark failed; inspect partial raw.json' }
} finally { Pop-Location }
