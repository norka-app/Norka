# Командная строка и ссылки norka://

Автоматизация выключена, пока её не включат в **Настройки → Функции**. Без флага команды завершаются с кодом 1 и не открывают локальный канал. Ссылки `norka://` в этом случае только показывают короткое сообщение в окне.

Канал — не TCP. На Windows это именованный канал, доступный только текущему пользователю. На Linux и macOS — unix-сокет с правами `0600`: в `$XDG_RUNTIME_DIR/norka/norka.sock`, если каталог времени выполнения задан, иначе рядом с `config.toml`. Рядом с конфигом лежит `automation.token` (права `0600`): каждая команда читает его и передаёт запущенному экземпляру. Чужой пользователь файл не прочитает.

Имя туннеля ищется так: точное совпадение без учёта регистра, затем id, затем единственный префикс. Если префикс подходит нескольким туннелям, команда пишет их имена и завершается с кодом 4.

## Отдельный клиент norka-cli

Команды `norka connect` … по-прежнему встроены в приложение. Рядом есть тонкая программа `norka-cli`: тот же канал, без окна, Wails, связки ключей и SSH. Справка и её собственные сообщения на английском. Текст ответа Norka в режиме человека клиент тоже пишет по-английски. `--json` печатает ответ канала как есть.

Установка:

```bash
brew install norka-app/tap/norka-cli
```

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka-cli
```

```bash
go install github.com/norka-app/Norka/cmd/norka-cli@latest
```

`go install …@latest` берёт последний тег `v*`. Пока тега с этим каталогом нет, та же команда с `@main`.

Пакеты deb и rpm лежат в том же выпуске GitHub, что и приложение: `norka-cli_<версия>_linux_<архитектура>.deb` и `.rpm`.

Чтобы команда дошла до туннеля, Norka должна быть запущена, а в **Настройки → Функции** включено «Автоматизация». Иначе код выхода 1, канал не открывается.

При старте Norka записывает путь к своему исполняемому файлу в `app-path` рядом с `config.toml`. `norka-cli connect`, если приложение не запущено, читает этот файл и запускает его скрытым, в трее. Нет файла или путь уже не существует — команда пишет, что Norka нужно запустить хотя бы один раз, и завершается с кодом 2. `disconnect`, `toggle`, `status` и `list` приложение сами не запускают.

В каждом запросе `norka-cli` есть поле `v` — версия протокола. Старый клиент без этого поля по-прежнему работает с новой Norka: так устроены уже выпущенные команды `norka`. Если номера не сходятся, команда завершается с кодом 8 и просит обновить Norka или `norka-cli`, а не выполняет действие вслепую.

## Команды

Встроенные, из каталога с `norka`:

```bash
norka connect <имя|id>
norka disconnect <имя|id>
norka toggle <имя|id>
norka status [--json]
norka list [--json]
```

Отдельный клиент:

```bash
norka-cli connect <имя|id>
norka-cli disconnect <имя|id>
norka-cli toggle <имя|id>
norka-cli status [--json]
norka-cli list [--json]
norka-cli --version
```

`connect`, если Norka не запущена, стартует её в трее без окна и затем подключает туннель. Встроенная команда запускает свой же файл. `norka-cli` запускает путь из `app-path`. `status` и `list` в этом случае пишут, что программа не запущена, и завершаются с кодом 2. `disconnect` и `toggle` сами программу не запускают.

```text
$ norka status
Norka запущена. Туннелей: 2, подключено: 1.

1	db	running	local	127.0.0.1:5432
2	api	stopped	dynamic	127.0.0.1:1080
```

`--json` печатает тот же ответ одним JSON-документом в stdout, в том числе при ошибке. Текст для человека при ошибке идёт в stderr.

## Коды выхода

| Код | Значение |
| --- | --- |
| 0 | успех |
| 1 | автоматизация выключена (Настройки → Функции) |
| 2 | Norka не запущена |
| 3 | туннель не найден |
| 4 | имя неоднозначно |
| 5 | неверные аргументы |
| 6 | нет связи с локальным каналом |
| 7 | команда туннеля не выполнена |
| 8 | разные версии Norka и norka-cli |

На Windows программа собрана без консоли. Команда присоединяется к консоли родителя (`AttachConsole`) и не открывает новое окно, поэтому `norka status` печатает текст и в cmd, и в PowerShell.

## Ярлык Windows

В поле «Объект» ярлыка:

```text
"C:\Users\you\AppData\Local\Programs\Norka\norka.exe" connect db
```

Двойной щелчок подключает туннель `db`. Если Norka не была запущена, она останется в трее.

## Псевдоним в оболочке

bash или zsh:

```bash
alias db='norka connect db'
```

PowerShell, в профиле (`$PROFILE`):

```powershell
function db { norka connect db }
```

## Ссылки

```text
norka://connect/db
norka://disconnect/db
norka://open/db
```

`open` только показывает туннель. `connect` и `disconnect` первый раз для этого туннеля спрашивают подтверждение. Галочка «Больше не спрашивать для этого туннеля» запоминается в `config.toml` (`automation_trusted`) и не стирается, когда флаг выключают. В меню `⋯` туннеля пункт «Скопировать ссылку norka://» копирует `norka://connect/…` и виден только при включённой автоматизации.

Схема регистрируется так:

- Windows: `HKCU\Software\Classes\norka`, только пока флаг включён; выключение ключ удаляет.
- macOS: `CFBundleURLTypes` в `Info.plist` (схема в `wails.json`).
- Linux: `MimeType=x-scheme-handler/norka` в `.desktop` пакетов deb, AppImage и tar.gz.

---

# CLI and norka:// links

Automation stays off until it is turned on in **Settings → Features**. While the flag is off, commands exit with code 1 and do not open the local channel. A `norka://` link then only shows a short message in the window.

The channel is not TCP. On Windows it is a named pipe limited to the current user. On Linux and macOS it is a unix socket with mode `0600`: `$XDG_RUNTIME_DIR/norka/norka.sock` when the runtime directory is set, otherwise next to `config.toml`. `automation.token` sits next to the config (mode `0600`). Each command reads it and sends it to the running instance. Another user cannot read the file.

A tunnel is matched by exact name first (case-insensitive), then by id, then by a unique prefix. If the prefix matches several tunnels, the command prints their names and exits with code 4.

## Standalone norka-cli

The `norka connect` … commands stay inside the app. `norka-cli` is a thin program for the same channel, without the window, Wails, the keychain, or SSH. Its help and its own messages are English. In human mode it also prints Norka's reply in English. `--json` prints the channel reply unchanged.

Install:

```bash
brew install norka-app/tap/norka-cli
```

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka-cli
```

```bash
go install github.com/norka-app/Norka/cmd/norka-cli@latest
```

`go install …@latest` follows the newest `v*` tag. Until a tag contains this package, use the same command with `@main`.

deb and rpm packages are attached to the same GitHub release as the app: `norka-cli_<version>_linux_<arch>.deb` and `.rpm`.

Norka has to be running, and **Settings → Features** has to have Automation on. Otherwise the command exits with code 1 and does not open the channel.

On startup Norka writes its executable path to `app-path` next to `config.toml`. If the app is not running, `norka-cli connect` reads that file and starts it hidden, in the tray. A missing file or a path that no longer exists asks you to start Norka once and exits with code 2. `disconnect`, `toggle`, `status`, and `list` do not start the app.

Every `norka-cli` request carries `v`, the protocol version. An older client that omits the field still works with a newer Norka, which is how the `norka` commands already shipped behave. When the numbers disagree, the command exits with code 8 and asks you to update Norka or `norka-cli` instead of guessing.

## Commands

Built into the app:

```bash
norka connect <name|id>
norka disconnect <name|id>
norka toggle <name|id>
norka status [--json]
norka list [--json]
```

Standalone client:

```bash
norka-cli connect <name|id>
norka-cli disconnect <name|id>
norka-cli toggle <name|id>
norka-cli status [--json]
norka-cli list [--json]
norka-cli --version
```

If Norka is not running, `connect` starts it hidden in the tray and then connects the tunnel. The built-in command starts its own executable. `norka-cli` starts the path stored in `app-path`. `status` and `list` say that it is not running and exit with code 2. `disconnect` and `toggle` do not start the app.

```text
$ norka status
Norka is running. Tunnels: 2, connected: 1.

1	db	running	local	127.0.0.1:5432
2	api	stopped	dynamic	127.0.0.1:1080
```

`--json` prints the same reply as one JSON document on stdout, including failures. Human-readable errors go to stderr.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | automation is off (Settings → Features) |
| 2 | Norka is not running |
| 3 | tunnel was not found |
| 4 | the name is ambiguous |
| 5 | invalid arguments |
| 6 | the local channel could not be reached |
| 7 | the tunnel command failed |
| 8 | Norka and norka-cli speak different versions |

On Windows the program is built without a console. A command attaches to the parent console (`AttachConsole`) and does not open a new window, so `norka status` prints in both cmd and PowerShell.

## Windows shortcut

Shortcut target:

```text
"C:\Users\you\AppData\Local\Programs\Norka\norka.exe" connect db
```

A double-click connects the tunnel `db`. If Norka was not running, it stays in the tray.

## Shell alias

bash or zsh:

```bash
alias db='norka connect db'
```

PowerShell, in the profile (`$PROFILE`):

```powershell
function db { norka connect db }
```

## Links

```text
norka://connect/db
norka://disconnect/db
norka://open/db
```

`open` only shows the tunnel. The first `connect` or `disconnect` for that tunnel asks for confirmation. “Don’t ask again for this tunnel” is stored in `config.toml` (`automation_trusted`) and is kept when the flag is turned off. A tunnel’s `⋯` menu item “Copy norka:// link” copies `norka://connect/…` and is shown only while automation is on.

The scheme is registered as follows:

- Windows: `HKCU\Software\Classes\norka`, only while the flag is on; turning it off deletes the key.
- macOS: `CFBundleURLTypes` in `Info.plist` (the scheme lives in `wails.json`).
- Linux: `MimeType=x-scheme-handler/norka` in the `.desktop` file shipped by the deb, AppImage, and tar.gz.
