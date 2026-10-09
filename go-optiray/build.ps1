param([ValidateSet('arm64','mipsle','all')][string]$Architecture='all')
$ErrorActionPreference='Stop'
$projectDir=$PSScriptRoot
$goPath=Join-Path (Split-Path $projectDir -Parent) 'research/toolchain/go/bin/go.exe'
if (!(Test-Path -LiteralPath $goPath)) { $goPath=(Get-Command go -ErrorAction Stop).Source }
$savedEnvironment=@{}
foreach($key in @('CGO_ENABLED','GOOS','GOARCH','GOMIPS')){$savedEnvironment[$key]=[Environment]::GetEnvironmentVariable($key,'Process')}
Push-Location $projectDir
try {
    # Host tests must run before setting Linux cross-compilation variables.
    foreach($key in @('GOOS','GOARCH','GOMIPS')){[Environment]::SetEnvironmentVariable($key,$null,'Process')}
    & $goPath test ./...
    if($LASTEXITCODE -ne 0){throw 'Go tests failed'}
    & $goPath vet ./...
    if($LASTEXITCODE -ne 0){throw 'Go vet failed'}
    $targets=if($Architecture -eq 'all'){@('arm64','mipsle')}else{@($Architecture)}
    $env:CGO_ENABLED='0';$env:GOOS='linux'
    New-Item -ItemType Directory -Force (Join-Path $projectDir 'build') | Out-Null
    foreach($target in $targets){
        $env:GOARCH=$target
        [Environment]::SetEnvironmentVariable('GOMIPS',$(if($target -eq 'mipsle'){'softfloat'}else{$null}),'Process')
        $output=Join-Path $projectDir "build/optiray-go-linux-$target"
        & $goPath build -trimpath -ldflags '-s -w' -o $output .
        if($LASTEXITCODE -ne 0){throw "Build failed for $target"}
		$engineDir=Join-Path (Split-Path $projectDir -Parent) 'research/OptiRout/src/backend'
		Push-Location $engineDir
		try { & $goPath build -mod=vendor -tags entware -trimpath -ldflags '-s -w' -o (Join-Path $projectDir "build/optiroutd-linux-$target") ./cmd/optiroutd; if($LASTEXITCODE -ne 0){throw "OptiRout build failed for $target"} } finally { Pop-Location }
        $digest=(Get-FileHash -LiteralPath $output -Algorithm SHA256).Hash.ToLowerInvariant()
        [IO.File]::WriteAllText("$output.sha256","$digest  optiray-go-linux-$target`n",[Text.UTF8Encoding]::new($false))
        Write-Host "Built $output"
    }
    Write-Host 'Development binaries only. Router was not updated.'
} finally {
    foreach($key in $savedEnvironment.Keys){[Environment]::SetEnvironmentVariable($key,$savedEnvironment[$key],'Process')}
    Pop-Location
}
