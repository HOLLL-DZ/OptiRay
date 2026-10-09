# OptiRay на Go

Функции Python 4.0.2 перенесены. Текущая поставка `1.0.3` — кандидат
для тестирования, проверенный на ARM64 Keenetic. Разработка и текущая рабочая установка переведены на Go. Состояние переноса: [PORTING.md](PORTING.md),

## Сборка на Windows

Из корня проекта:

```powershell
./go-optiray/package.ps1
```

Команда запускает тесты и `go vet`, затем собирает статические ARM64 и MIPSel
бинарники и архивы с SHA256 в `go-optiray/build/packages/`.
Для одной архитектуры добавьте `-Architecture arm64`.
Сборка не загружает файлы на роутер и не меняет его установку.

## Установка и обновление на Entware

Серверный установщик: `http://example.com/optiray_go/install.sh`.
Для чистого Entware скопируйте весь блок:

```sh
opkg update && opkg install curl ca-bundle &&
curl -fL http://example.com/optiray_go/install.sh -o /tmp/install-optiray-go.sh &&
sh /tmp/install-optiray-go.sh http://example.com/optiray_go
```

Если установщик уже скачан, достаточно последней команды. Он сам выбирает
архитектуру, проверяет SHA256 и устанавливает недостающие `iptables`, `ip-full`
и сертификаты через `opkg` до остановки текущей службы. Python не устанавливается.

После установки или обновления служба **S99optiray-go** запускается автоматически.
Успех выводится после проверки HTTP, маршрутизации и выбранного ядра.
Адрес домашнего моста определяется автоматически: для роутера 192.168.10.1
будет показана ссылка `http://192.168.10.1:3883`. На свежей установке этот же адрес
попадает в настройки прокси и ссылку на политики Keenetic. Обновление не меняет
настроенный пользователем адрес.

Если найдена Python-версия OptiRay, установщик переносит её настройки, HWID,
авторизацию, правила и имеющиеся ядра в отдельный `/opt/etc/optiray-go`.
Её автозапуск и firewall hook сохраняются в `install-state/` и отключаются.
Останавливаются только процессы с проверенными путями OptiRay и PID движка;
`killall` не используется. Исходные данные Python остаются на месте.

При ошибке новой службы установщик возвращает прежние файлы и ранее работавшую
службу. Путь к диагностике и резервной копии печатается в консоли.

Управление Go:

```sh
/opt/etc/init.d/S99optiray-go status
/opt/etc/init.d/S99optiray-go restart
```

Xray/Hysteria на свежем устройстве устанавливаются во вкладке «Ядра».
Go-компилятор на роутере не нужен. Удаление:

```sh
sh /opt/etc/optiray-go/uninstall.sh
```

Удаляются собственные файлы Go и управляемый прокси. Если установка заменила
работавшую Python-версию, её сохранённые файлы автозапуска возвращаются,
а служба восстанавливается. Чужие подключения не удаляются. Пакеты Entware,
которые могут использовать другие приложения, автоматически не удаляются.

## Интерфейс без пересборки

Редактируемые исходники: `assets/page.html`, `assets/style.css`, `assets/app.js`.
Служба читает установленные `/opt/etc/optiray-go/ui/` при каждом запросе:
после правки достаточно обновить страницу. При указании `--ui-dir` нужны все три
файла. Без этого параметра сервер использует встроенные копии.

Python 4.0.2 сохранён как эталон. Новую разработку ведём в Go; изменения логики
компилируются на компьютере и заменяют один бинарник менеджера. Интерфейс можно
менять отдельно. `tools/extract_assets.py` служит только для повторного импорта
исходного интерфейса и не запускается на роутере.

Temporary installation files and the full rollback snapshot now use `/tmp` by default (RAM on Keenetic), keeping them off small Entware flash storage. The installer checks temporary and destination free space before stopping the service, frees replaced manager/engine binaries before copying replacements, and keeps installed cores and geodata. To use a USB directory instead, run `OPTIRAY_INSTALL_TMPDIR=/path/on/usb sh install.sh REPOSITORY_URL`. A failed update retains its diagnostic snapshot in the temporary directory until cleanup or reboot; copy it elsewhere before reboot if needed.

HWID is stored only in `subs.json` and included in settings/full backups. On upgrade, a legacy `hwid.txt` is imported only if settings lack an HWID, then removed after successful persistence. Existing settings and restored backups take precedence over stale text files. Legacy transaction journal recovery remains supported.

The main OptiRay power switch persists core state and disables OptiRout on switch-off. OptiRout can then be enabled independently using its modal and Save, including routes through other interfaces while the proxy core remains off. The routing switch state is included in full backups.
