# Фоновый режим

Фоновый режим позволяет держать туннели без открытого окна. Их ведёт отдельный процесс `norkad`: тот же `config.toml`, что у окна, автозапуск туннелей и переподключение после сна. Окна, трея и связки ключей у демона нет.

Переключатель называется «Фоновый режим». Он лежит в **Настройки → Функции** и по умолчанию выключен. Каталог флагов — в [FEATURES.md](FEATURES.md).

Пока переключатель выключен, окно держит туннели само, как раньше. `norkad` из окна не запускается, пункт трея «Выйти и остановить туннели» не появляется. Запущенный вручную `norkad` пишет, что режим выключен, и завершается с кодом 10. `--force` эту проверку обходит.

## Как окно подключается

Когда флаг включён, окно становится клиентом `norkad`. Если демон уже работает, окно к нему подключается. Если нет — ищет `norkad` и запускает его с тем же `config.toml`.

Поиск идёт рядом с программой, внутри пакета `.app` на macOS и затем в `PATH`. Поэтому демон из Homebrew или Scoop находится, даже если окно и `norkad` ставились отдельно.

Подключение идёт по локальному каналу, протокол 2: `hello` и `subscribe`. Пока демон жив, окно не берёт блокировку движка. Закрытие окна демон не останавливает, туннели остаются. Обычный пункт трея «Выход» тоже оставляет `norkad` работать.

Если `norkad` не найден или не поднялся, окно остаётся владельцем движка и пишет об этом в настройках. Если движок уже занят другим процессом, это окно второй набор туннелей не поднимает.

Канал для окна открыт и без «Автоматизации». Команды первой версии по-прежнему получают отказ, пока автоматизация выключена. Формат запросов — в [IPC.md](IPC.md).

## Трей

При включённом флаге в трее есть «Выйти и остановить туннели». Пункт останавливает `norkad` и затем закрывает окно. Подсказка: «Остановить фоновый процесс и закрыть окно». Выключение флага пункт прячет.

## Настройки → Общие

При включённом флаге в **Настройки → Общие** появляется секция «Фоновый режим». Если включено «Окно без рамки», секция лежит во вкладке «Общие». Без этого флага она в той же карточке «Общие».

В секции видно, запущен ли процесс: «Запущен» или «Не запущен», PID и версия. Если туннели сейчас держит само окно, это тоже написано здесь. Кнопка «Остановить фоновый процесс» просит `norkad` отпустить движок. Туннели в демоне закрываются, и окно снова держит их само.

Переключатель «Запускать при входе в систему» ставит `norkad` в автозагрузку сеанса: демон стартует вместе со входом и поднимает туннели без окна. Права администратора не нужны. На Linux это systemd `--user`, а если он недоступен — запись XDG autostart. На macOS — LaunchAgent. На Windows — значение в `HKCU\...\Run`, не служба.

Выключение флага «Фоновый режим» снимает эту автозагрузку и спрашивает, остановить ли `norkad` сейчас. «Остановить» закрывает его туннели, и окно снова держит их само. «Оставить» демона не трогает.

## Пароли

Демон пароль не спрашивает. При своём запуске он пропускает туннель, которому нужен пароль или passphrase ключа, и пишет об этом в журнал. Ключ без passphrase и SSH Agent поднимаются без окна.

Если такой туннель запускает пользователь из уже подключённого окна, окно читает секрет из связки ключей и передаёт его в `control.secrets` на один `start` или `restart`. В `config.toml` секрет не пишется. В ответе, снимке состояния и журнале его нет. Подробности поля — в [IPC.md](IPC.md).

## Как поставить и запустить norkad

`norkad` лежит в тех же пакетах, что и `norka-cli`: cask Homebrew, манифест Scoop, архивы выпуска, deb и rpm. В каждом два файла, `norka-cli` и `norkad`. `go install` клиента демон не ставит.

```bash
brew install norka-app/tap/norka-cli
```

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka-cli
```

Архив, zip, deb или rpm — в том же выпуске GitHub, что и приложение. Имена и проверка сборки — в [CLI.md](CLI.md).

Окно само запускает демон, когда флаг включён. Вручную:

```bash
norkad [--config PATH] [--foreground] [--force] [--version]
```

`--config` — путь к `config.toml`. Без этого аргумента берётся тот же файл, что у окна. `--foreground` пишет журнал в stderr. Иначе журнал — `norkad.log` рядом с конфигом. `--force` запускает демон и при выключенном фоновом режиме. `--version` печатает версию.

Коды выхода демона: 0 — остановка по сигналу, туннели закрыты; 2 — неверные флаги; 9 — движок уже держит другой процесс, в сообщении его pid и kind (`gui` или `daemon`); 10 — фоновый режим выключен; 1 — любая другая ошибка.

## Команды norka-cli

Клиент говорит с тем, кто держит туннели: с окном или с `norkad`. Для команд клиента в **Настройки → Функции** нужна «Автоматизация». Без неё `norka-cli` завершается с кодом 1 и до канала не доходит. `daemon` программу сам не запускает. Коды выхода и остальные команды — в [CLI.md](CLI.md).

```bash
norka-cli status --json
norka-cli daemon stop [--force]
norka-cli daemon handover
```

`status --json` говорит на протоколе 2. В ответе есть приветствие (`owner` — `gui` или `daemon`, pid и версия приложения) и снимок туннелей, джамперов, групп и статистики. Паролей в ответе нет. Обычный `status` без `--json` остаётся коротким текстом.

`daemon stop` просит владельца остановить туннели и выйти. `norkad` останавливается и без `--force`. Окно эту команду игнорирует, пока нет `--force`. `daemon handover` просит остановить туннели и отпустить блокировку движка, чтобы её занял следующий процесс.

---

# Background mode

Background mode keeps tunnels up without the window. A separate process, `norkad`, runs them: the same `config.toml` as the window, autostart tunnels, and reconnect after sleep. The daemon has no window, no tray, and no keychain.

The switch is labeled Background mode. It lives in **Settings → Features** and is off by default. The flag catalog is in [FEATURES.md](FEATURES.md).

While the switch is off, the window hosts the tunnels itself, as before. The window does not start `norkad`, and the tray item “Quit and stop tunnels” is absent. A `norkad` you start by hand says that background mode is off and exits with code 10. `--force` skips that check.

## How the window attaches

When the flag is on, the window is a client of `norkad`. If the daemon is already running, the window attaches to it. If not, the window looks for `norkad` and starts it with the same `config.toml`.

The search looks next to the app, inside the `.app` bundle on macOS, and then on `PATH`. A Homebrew or Scoop install is found even when the window and `norkad` were installed separately.

Attach uses the local channel, protocol 2: `hello` and `subscribe`. While the daemon is alive, the window does not take the engine lock. Closing the window does not stop the daemon, so the tunnels keep running. The ordinary tray item “Quit” also leaves `norkad` running.

If `norkad` is missing or does not start, the window stays the engine owner and says so in Settings. If another process already holds the engine, this window does not start a second set of tunnels.

The window’s channel stays open even when Automation is off. Dialect 1 commands still get a refusal until automation is on. The request format is in [IPC.md](IPC.md).

## Tray

With the flag on, the tray has “Quit and stop tunnels”. That item stops `norkad` and then closes the window. The tooltip is “Stop the background process and close the window”. Turning the flag off hides the item.

## Settings → General

With the flag on, **Settings → General** shows a Background mode section. When Frameless window is on, the section is on the General tab. Without that flag it is in the same General card.

The section shows whether the process is running: “Running” or “Not running”, plus the PID and version. If this window is hosting the tunnels, that is written here too. “Stop background process” asks `norkad` to release the engine. Tunnels in the daemon close, and this window hosts them again.

“Start at login” registers `norkad` for the user session: the daemon starts at login and brings tunnels up without the window. No administrator rights. On Linux that is systemd `--user`, or an XDG autostart entry when systemd `--user` is not available. On macOS it is a LaunchAgent. On Windows it is an `HKCU\...\Run` value, not a service.

Turning Background mode off removes that login entry and asks whether to stop `norkad` now. Stop closes its tunnels, and the window hosts them again. Leave it does not touch the daemon.

## Passwords

The daemon never asks for a password. On its own start it skips a tunnel that needs a password or a key passphrase, and writes that to the log. A key without a passphrase, and SSH Agent, start without the window.

If you start such a tunnel from a window that is already attached, the window reads the secret from the keychain and sends it in `control.secrets` for one `start` or `restart`. The secret is not written to `config.toml`. It is not in the reply, the state snapshot, or the log. The field is described in [IPC.md](IPC.md).

## Install and run norkad

`norkad` ships in the same packages as `norka-cli`: the Homebrew cask, the Scoop manifest, the release archives, and the deb and rpm packages. Each one contains both files, `norka-cli` and `norkad`. `go install` of the client does not install the daemon.

```bash
brew install norka-app/tap/norka-cli
```

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka-cli
```

The archive, zip, deb, or rpm is on the same GitHub release as the app. Names and attestation are in [CLI.md](CLI.md).

The window starts the daemon itself when the flag is on. By hand:

```bash
norkad [--config PATH] [--foreground] [--force] [--version]
```

`--config` is the path to `config.toml`. Without that argument, `norkad` uses the same file as the window. `--foreground` writes the log to stderr. Otherwise the log is `norkad.log` next to the config. `--force` starts the daemon even when background mode is off. `--version` prints the version.

Daemon exit codes: 0 — stopped after a signal, tunnels closed; 2 — invalid flags; 9 — another process holds the engine, and the message includes its pid and kind (`gui` or `daemon`); 10 — background mode is off; 1 — any other error.

## norka-cli commands

The client talks to whoever holds the tunnels: the window or `norkad`. Client commands need Automation on in **Settings → Features**. Without it, `norka-cli` exits with code 1 and does not open the channel. `daemon` does not start the app. Exit codes and the other commands are in [CLI.md](CLI.md).

```bash
norka-cli status --json
norka-cli daemon stop [--force]
norka-cli daemon handover
```

`status --json` speaks protocol 2. The reply has a hello (`owner` is `gui` or `daemon`, plus the pid and the app version) and a snapshot of tunnels, jumpers, groups, and stats. Passwords are not in the reply. Plain `status` without `--json` stays the short text.

`daemon stop` asks the owner to stop its tunnels and exit. `norkad` stops even without `--force`. A window ignores that command unless `--force` is set. `daemon handover` asks the owner to stop its tunnels and release the engine lock so the next process can take it.
