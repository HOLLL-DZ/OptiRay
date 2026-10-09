param([ValidateSet('arm64','mipsle','all')][string]$Architecture='all')
$ErrorActionPreference='Stop'
& (Join-Path $PSScriptRoot 'build.ps1') -Architecture $Architecture
$versionSource=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'main.go'))
$match=[regex]::Match($versionSource,'const version = "([^"]+)"')
if(!$match.Success){throw 'Missing Go version'}
$packageVersion=$match.Groups[1].Value
if($packageVersion -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$'){throw 'Invalid package version'}
$stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
$packageRoot=Join-Path $PSScriptRoot "build/packages/$stamp"
New-Item -ItemType Directory -Force -Path $packageRoot | Out-Null
$engineRoot=Join-Path (Split-Path $PSScriptRoot -Parent) 'research/OptiRout'
$sourceArchive=Join-Path $packageRoot 'optirout-source.tar.gz'
& tar -czf $sourceArchive -C $engineRoot src/backend LICENSE CONTRIBUTORS.md OPTIRAY-BUILD.md
if($LASTEXITCODE -ne 0){throw 'Source archive failed'}
$targets=if($Architecture -eq 'all'){@('arm64','mipsle')}else{@($Architecture)}
foreach($architecture in $targets){
    $folder=Join-Path $packageRoot "OptiRay-Go-$packageVersion-$architecture"
    New-Item -ItemType Directory -Force -Path (Join-Path $folder 'bin'),(Join-Path $folder 'ui') | Out-Null
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "build/optiray-go-linux-$architecture") -Destination (Join-Path $folder 'bin/optiray')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "build/optiroutd-linux-$architecture") -Destination (Join-Path $folder 'bin/optiroutd')
    foreach($name in @('page.html','style.css','app.js')){Copy-Item -LiteralPath (Join-Path $PSScriptRoot "assets/$name") -Destination (Join-Path $folder "ui/$name")}
    foreach($name in @('install.sh','uninstall.sh','S99optiray-go','100-optiray-go-routing')){Copy-Item -LiteralPath (Join-Path $PSScriptRoot "server-files/$name") -Destination $folder}
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'README.md') -Destination $folder
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'PORTING.md') -Destination $folder
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'CHANGELOG.md') -Destination $folder
    foreach($name in @('NOTICE.optirout','AUTHORS.optirout','LICENSE.optirout')){Copy-Item -LiteralPath (Join-Path $PSScriptRoot "assets/$name") -Destination $folder}
    Copy-Item -LiteralPath $sourceArchive -Destination $folder
    [IO.File]::WriteAllText((Join-Path $folder 'ARCH'),"$architecture`n",[Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $folder 'VERSION'),"$packageVersion`n",[Text.UTF8Encoding]::new($false))
    $lines=@()
    foreach($file in Get-ChildItem -LiteralPath $folder -File -Recurse){
        $relative=$file.FullName.Substring($folder.Length+1).Replace('\','/')
        $lines+="$( (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() )  $relative"
    }
    [IO.File]::WriteAllText((Join-Path $folder 'SHA256SUMS'),(($lines -join "`n")+"`n"),[Text.UTF8Encoding]::new($false))
    $archive=Join-Path $packageRoot "OptiRay-Go-$packageVersion-$architecture.tar.gz"
    & tar -czf $archive -C $folder .
    if($LASTEXITCODE -ne 0){throw "Package failed: $architecture"}
    Write-Host "Package: $archive"
    $archiveDigest=(Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText("$archive.sha256","$archiveDigest  $(Split-Path $archive -Leaf)`n",[Text.UTF8Encoding]::new($false))
}
Write-Host 'Packages are local. Production router installation was not replaced.'
