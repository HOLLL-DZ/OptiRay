param([Parameter(Mandatory=$true)][ValidatePattern('^[0-9]+\.[0-9]+\.[0-9]+$')][string]$Version)
$ErrorActionPreference='Stop'
$utf8=[Text.UTF8Encoding]::new($false)
$sourceVersion=[regex]::Match([IO.File]::ReadAllText((Join-Path $PSScriptRoot 'main.go')),'const version = "([^"]+)"').Groups[1].Value
if($sourceVersion -ne $Version){throw "Go source version $sourceVersion does not match $Version"}
$release=Join-Path (Split-Path $PSScriptRoot -Parent) "releases/$Version"
if(Test-Path -LiteralPath $release){throw "Release already exists: $release"}
& (Join-Path $PSScriptRoot 'package.ps1')
$packages=(Get-ChildItem -LiteralPath (Join-Path $PSScriptRoot 'build/packages') -Directory | Sort-Object Name -Descending | Select-Object -First 1).FullName
$server=Join-Path $release 'Server File'
New-Item -ItemType Directory -Path $server -Force | Out-Null
foreach($arch in @('arm64','mipsle')){
 $name="OptiRay-Go-$Version-$arch.tar.gz"
 foreach($suffix in @('','.sha256')){Copy-Item -LiteralPath (Join-Path $packages "$name$suffix") -Destination $server}
}
$bootstrap=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'server-files/download-install.sh.in')).Replace('@VERSION@',$Version)
[IO.File]::WriteAllText((Join-Path $server 'install.sh'),$bootstrap,$utf8)
[IO.File]::WriteAllText((Join-Path $server 'VERSION'),"$Version`n",$utf8)
foreach($name in @('README.md','PORTING.md','CHANGELOG.md')){Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination $release}
[IO.File]::WriteAllText((Join-Path $release 'VERSION'),"$Version`n",$utf8)
$lines=@()
foreach($file in Get-ChildItem -LiteralPath $server -File){$lines+="$( (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() )  $($file.Name)"}
[IO.File]::WriteAllText((Join-Path $server 'SHA256SUMS'),(($lines -join "`n")+"`n"),$utf8)
$instructions=@"
# Серверный выпуск OptiRay Go $Version

Загрузите содержимое папки ``Server File`` в отдельный каталог сервера,
``/var/www/example.com/optiray_go``. Не заменяйте репозиторий Python/IPK этими файлами.
Публикация на сервер этим скриптом не выполняется.

Установка из Entware-консоли (подставьте адрес своего каталога):

``````sh
opkg update && opkg install curl ca-bundle &&
curl -fL http://example.com/optiray_go/install.sh -o /tmp/install-optiray-go.sh &&
sh /tmp/install-optiray-go.sh http://example.com/optiray_go
``````

Установщик проверяет SHA256, выбирает ARM64 или MIPSel и устанавливает Go отдельно
в ``/opt/etc/optiray-go``. Сервис запускается автоматически после установки и проверки готовности;
зависимости и адрес роутера определяются автоматически. Подробности — в README.md. Python-зависимости не нужны.
Номер сервера, VERSION внутри архивов и версия Go-бинарников: $Version.
MIPSel собран; физически проверена ARM64-версия. Статус поставки — тестовый.
"@
[IO.File]::WriteAllText((Join-Path $release 'SERVER.md'),$instructions.Replace("`r`n","`n"),$utf8)
$zip=Join-Path (Split-Path $release -Parent) "OptiRay-Go-$Version.zip"
if(Test-Path -LiteralPath $zip){throw "Release archive already exists: $zip"}
Compress-Archive -LiteralPath $release -DestinationPath $zip
$digest=(Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText("$zip.sha256","$digest  $(Split-Path $zip -Leaf)`n",$utf8)
Write-Host "Server release: $server"
Write-Host "Release archive: $zip"
