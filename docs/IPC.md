# Локальный канал автоматизации

Канал — не TCP. Его слушает процесс, который держит движок: окно или `norkad`. Транспорт и проверка прав не менялись.

- Windows: именованный канал только для текущего пользователя (`\\.\pipe\norka-<SID>`).
- Linux и macOS: unix-сокет с правами `0600`. Это `$XDG_RUNTIME_DIR/norka/norka.sock`, если каталог времени выполнения задан, иначе `norka.sock` рядом с `config.toml`.
- Рядом с конфигом лежит `automation.token` (права `0600`). Клиент читает его и кладёт в поле `token` каждого запроса. Сервер сравнивает токен и не пишет его в журнал. Чужой пользователь файл не прочитает.

Канал открывается, только когда в **Настройки → Функции** включена автоматизация. Иначе `norka-cli` завершается с кодом 1 и никуда не подключается.

## Кадр

Один JSON-документ на строку, в конце перевод строки. Запрос — первая строка соединения. Ответ — следующая. Для всех команд, кроме `subscribe`, сервер после ответа закрывает соединение.

Размер одной строки — не больше 1 МиБ. Поля, которых нет в этой версии, сторона игнорирует.

## Версии

Поле `v` в запросе — диалект клиента.

| `v` | Кто это | Что делает сервер |
| --- | --- | --- |
| нет или `0` | уже выпущенные команды `norka` | принимает как раньше: `connect`, `disconnect`, `toggle`, `status`, `list` |
| `1` | старый `norka-cli` | то же самое. `hello` не нужен |
| `2` | этот выпуск | старые команды и новые: `hello`, `state`, `subscribe`, `control`, `shutdown`, `handover` |

Ответ всегда содержит `"v": 2` — диалект этого приложения. Старый клиент считает ответ без `v` слишком старым приложением и не выполняет команду. Ответ с `v: 2` для него обычный успех или обычная ошибка.

Если клиент прислал `v` больше 2, сервер не выполняет команду и отвечает `update_app` (код выхода 8): «This norka-cli is newer than Norka. Update Norka.» Если когда-нибудь минимум поднимется выше присланного номера, ответ будет `update_cli`.

Новые команды при `v` меньше 2 — это обычный `bad_request`, как любая неизвестная команда первой версии. Старый клиент их не шлёт.

## Конверт

Запрос:

```json
{"v":2,"token":"<automation.token>","op":"hello","client":{"name":"norka-cli","version":"dev","protocol":2}}
```

`client` есть у `hello` и у команд `norka-cli`. Это имя, версия и номер протокола клиента. Токен туда не входит.

Ответ:

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true}
```

`running` — истина, когда ответ пришёл от живого процесса. `norka-cli` сам ставит `false`, если сокет или канал не открылся. Коды `code` и `exitCode` те же, что в [CLI.md](CLI.md): 0 успех, 1 автоматизация выключена, 2 программа не запущена, 3 не найдено, 4 имя неоднозначно, 5 неверные аргументы, 6 нет связи, 7 команда не выполнена, 8 разные версии.

Дополнительно у второй версии:

| `code` | Когда |
| --- | --- |
| `ignored` | окно проигнорировало `shutdown`, потому что не было `force` |
| `timeout` | `handover` не дождался остановки туннелей. `engine.lock` всё ещё занят |

## hello

Клиент представляется. Сервер отвечает, кто держит `engine.lock`.

```json
{"v":2,"token":"…","op":"hello","client":{"name":"norka-cli","version":"1.5.0","protocol":2}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"hello":{"protocol_version":2,"min_client_version":1,"owner":"daemon","pid":1234,"version":"1.5.0"}}
```

- `protocol_version` — диалект сервера, сейчас 2.
- `min_client_version` — самый старый нумерованный диалект, который сервер ещё принимает. Сейчас 1. Запрос без `v` принимается и при более высоком минимуме.
- `owner` — `gui` или `daemon`.
- `pid` — процесс, записанный в `engine.lock`.
- `version` — версия приложения из той же записи.

Если этот процесс слушает канал, но блокировку не держит, `owner` и `pid` берутся из файла блокировки, когда его можно прочитать.

## state

Полный снимок, которым окно может повторить состояние демона. Секретов в нём нет: у джампера нет пароля, только `hasSecret`.

```json
{"v":2,"token":"…","op":"state"}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"state":{"tunnels":[{"id":1,"name":"db","status":"running","mode":"local","localHost":"127.0.0.1","localPort":5432}],"jumpers":[{"id":1,"name":"bastion","host":"bastion.example","port":22,"user":"norka","authType":"ssh_agent"}],"groups":[{"id":1,"name":"prod"}],"stats":[]}}
```

`stats` есть только когда включена статистика туннелей. Выключенный флаг убирает поле целиком. Форма одного элемента — та же, что у счётчиков в интерфейсе (`id`, время на связи, переподключения, байты).

`norka-cli status --json` делает `hello`, затем `state`, и печатает оба в одном документе.

## subscribe

Та же безопасность, что у остальных команд. Соединение не закрывается после первого ответа.

1. Клиент шлёт `{"v":2,"token":"…","op":"subscribe"}`.
2. Первая строка ответа — снимок, как у `state`.
3. Дальше по строке на событие, пока клиент не закроет соединение.

Подписчик регистрируется до снимка, так что событие, случившееся во время снимка, придёт следом. Повтор того же состояния безопасен. Медленный подписчик может пропустить строку: буфер не ждёт его, чтобы не задерживать остановку туннеля. После пропуска нужно снова прочитать `state`.

Типы, которые нужны окну, чтобы повторять демон:

| `event.type` | Когда | Поля |
| --- | --- | --- |
| `status` | сменился статус туннеля | `tunnel` — тот же вид, что в снимке |
| `log` | строка журнала владельца | `level`, `line` |
| `notification` | уведомление, которое владелец показал бы в системе | `title`, `body`, `tunnelId` |
| `config` | владелец сохранил или удалил туннель, джампер или группу | `state` — новый снимок |

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"status","tunnel":{"id":1,"name":"db","status":"stopped","mode":"local"}}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"log","level":"INFO","line":"daemon ready"}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"notification","title":"Norka","body":"db dropped","tunnelId":1}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"config","state":{"tunnels":[],"jumpers":[],"groups":[]}}}
```

## control

Пишет и запускает только владелец `engine.lock`. Клиент, который блокировку не держит, сам конфиг не меняет: он просит владельца. Владелец пишет туннели, джамперы и группы через `conf.Storage`, под его межпроцессной блокировкой конфига. Процесс без `engine.lock` на `control` отвечает `failed` и текстом `this process does not hold the engine lock`.

```json
{"v":2,"token":"…","op":"control","control":{"action":"start","kind":"tunnel","id":1}}
```

| `action` | `kind` | Что делает владелец |
| --- | --- | --- |
| `start`, `stop`, `restart` | `tunnel` | запуск, остановка или остановка и снова запуск. Нужен `id` либо `target` у запроса (имя или id, те же правила, что у `norka connect`) |
| `save` | `tunnel`, `jumper`, `group` | `id` 0 или поле без id создаёт. Положительный `id` обновляет. Тело — `tunnel`, `jumper` или `group`, те же поля, что у окна |
| `delete` | `tunnel`, `jumper`, `group` | удаляет по `id` |

Успешные `save` и `delete` возвращают новый `state` и шлют событие `config` подписчикам. `start`, `stop` и `restart` возвращают `tunnels` с одним туннелем; смена статуса дополнительно уходит в `subscribe` как `status`.

Пароль джампера в запросе `save` допустим: его забирает связка ключей, в снимок и в ответ он не попадает.

## shutdown

Владелец останавливает свои туннели и завершает процесс.

```json
{"v":2,"token":"…","op":"shutdown"}
```

```json
{"v":2,"token":"…","op":"shutdown","force":true}
```

- Демон останавливается и без `force`.
- Окно (`owner` = `gui`) отвечает `ignored` и ничего не останавливает, пока нет `"force": true`.
- `norka-cli daemon stop` шлёт запрос без `force`. `norka-cli daemon stop --force` ставит `force`.

Ответ `ok` значит, что остановка принята. Процесс отпускает канал и `engine.lock` уже после этой строки.

## handover

Владелец останавливает туннели и отпускает `engine.lock`, чтобы вызвавший процесс мог сделать `Acquire`. Это не то же самое, что `shutdown`: блокировка освобождается до ответа, с предельным временем ожидания.

```json
{"v":2,"token":"…","op":"handover","timeout_ms":10000}
```

`timeout_ms` — сколько ждать остановки туннелей. Ноль или пропуск поля означает 10 секунд. `norka-cli daemon handover` шлёт 10 секунд.

Успех:

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"message":"engine lock released"}
```

После этой строки блокировка уже свободна, и процесс завершается, чтобы отдать сокет следующему владельцу.

Если туннели не остановились за отведённое время:

```json
{"v":2,"ok":false,"code":"timeout","exitCode":7,"running":true,"message":"handover timed out after 10s; the owner still holds the engine lock"}
```

Блокировка при этом остаётся у владельца. Туннели могли ещё останавливаться.

Окно `handover` не игнорирует: отдать блокировку демону можно и без `--force`. Процесс, который канал слушает, но блокировку не держит, отвечает той же ошибкой, что и на `control`.

---

# Automation IPC

The channel is not TCP. The process that holds the engine listens: the window, or `norkad`. The transport and its permission checks are unchanged.

- Windows: a named pipe limited to the current user (`\\.\pipe\norka-<SID>`).
- Linux and macOS: a unix socket with mode `0600`. That is `$XDG_RUNTIME_DIR/norka/norka.sock` when the runtime directory is set, otherwise `norka.sock` next to `config.toml`.
- `automation.token` sits next to the config (mode `0600`). The client reads it and puts it in `token` on every request. The server compares the token and never logs it. Another user cannot read the file.

The channel is open only while Automation is on in **Settings → Features**. Otherwise `norka-cli` exits 1 and does not connect.

## Framing

One JSON document per line, terminated by a newline. The request is the first line of the connection. The response is the next line. For every command except `subscribe`, the server closes the connection after the response.

One line is at most 1 MiB. A peer ignores fields it does not know.

## Versions

`v` on the request is the client's dialect.

| `v` | Who | What the server does |
| --- | --- | --- |
| omitted or `0` | the `norka` commands already shipped | the original `connect`, `disconnect`, `toggle`, `status`, `list` |
| `1` | an older `norka-cli` | the same. No `hello` |
| `2` | this release | those commands, plus `hello`, `state`, `subscribe`, `control`, `shutdown`, `handover` |

Every response carries `"v": 2`, the dialect this app speaks. An old client treats a response with no `v` as an app that is too old and does not run the command. A response with `v: 2` is an ordinary success or an ordinary error for that client.

A client `v` above 2 does not run. The server answers `update_app` (exit 8): "This norka-cli is newer than Norka. Update Norka." If the minimum ever moves past the number the client sent, the answer is `update_cli`.

A new command with `v` below 2 is a normal `bad_request`, the same as any unknown command in dialect 1. Old clients do not send them.

## Envelope

Request:

```json
{"v":2,"token":"<automation.token>","op":"hello","client":{"name":"norka-cli","version":"dev","protocol":2}}
```

`client` is sent with `hello` and with `norka-cli` commands. It is the client's name, version, and protocol number. The token is not part of it.

Response:

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true}
```

`running` is true when a live process wrote the response. `norka-cli` sets it false when the socket or pipe is not there. `code` and `exitCode` are the same pairs as in [CLI.md](CLI.md): 0 success, 1 automation is off, 2 the app is not running, 3 not found, 4 ambiguous name, 5 bad arguments, 6 the channel could not be reached, 7 the command failed, 8 the dialects differ.

Dialect 2 adds:

| `code` | When |
| --- | --- |
| `ignored` | the window ignored `shutdown` because `force` was not set |
| `timeout` | `handover` did not finish stopping tunnels. `engine.lock` is still held |

## hello

The client introduces itself. The server says who holds `engine.lock`.

```json
{"v":2,"token":"…","op":"hello","client":{"name":"norka-cli","version":"1.5.0","protocol":2}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"hello":{"protocol_version":2,"min_client_version":1,"owner":"daemon","pid":1234,"version":"1.5.0"}}
```

- `protocol_version` is the server dialect, currently 2.
- `min_client_version` is the oldest numbered dialect this server still accepts. Currently 1. A request that omits `v` stays accepted even if that minimum moves.
- `owner` is `gui` or `daemon`.
- `pid` is the process recorded in `engine.lock`.
- `version` is the application version from that same record.

If this process is listening but does not hold the lock, `owner` and `pid` come from the lock file when it can be read.

## state

A full snapshot a window can use to mirror the daemon. It has no secrets: a jumper has no password, only `hasSecret`.

```json
{"v":2,"token":"…","op":"state"}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"state":{"tunnels":[{"id":1,"name":"db","status":"running","mode":"local","localHost":"127.0.0.1","localPort":5432}],"jumpers":[{"id":1,"name":"bastion","host":"bastion.example","port":22,"user":"norka","authType":"ssh_agent"}],"groups":[{"id":1,"name":"prod"}],"stats":[]}}
```

`stats` is present only while tunnel stats are enabled. Turning the flag off removes the field. One element has the same shape as the counters in the UI (`id`, time connected, reconnects, bytes).

`norka-cli status --json` sends `hello`, then `state`, and prints both in one document.

## subscribe

The same transport checks as every other command. The connection stays open after the first response.

1. The client sends `{"v":2,"token":"…","op":"subscribe"}`.
2. The first response line is a snapshot, the same shape as `state`.
3. Later lines are events, one per line, until the client closes the connection.

The subscriber is registered before the snapshot, so an event during the snapshot arrives after it. Applying the same state twice is safe. A slow subscriber can miss a line: the buffer does not wait, so it cannot stall a tunnel stopping. After a gap, read `state` again.

Event types the window needs to mirror the daemon:

| `event.type` | When | Fields |
| --- | --- | --- |
| `status` | a tunnel's status changed | `tunnel`, the same view as in the snapshot |
| `log` | a log line from the owner | `level`, `line` |
| `notification` | a notification the owner would show | `title`, `body`, `tunnelId` |
| `config` | the owner saved or deleted a tunnel, jumper, or group | `state`, the new snapshot |

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"status","tunnel":{"id":1,"name":"db","status":"stopped","mode":"local"}}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"log","level":"INFO","line":"daemon ready"}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"notification","title":"Norka","body":"db dropped","tunnelId":1}}
```

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"event":{"type":"config","state":{"tunnels":[],"jumpers":[],"groups":[]}}}
```

## control

Only the `engine.lock` owner writes or starts tunnels. A process that does not hold the lock never writes the config itself: it asks the owner. The owner writes tunnels, jumpers, and groups through `conf.Storage`, under that storage's cross-process lock. A process without `engine.lock` answers `control` with `failed` and the text `this process does not hold the engine lock`.

```json
{"v":2,"token":"…","op":"control","control":{"action":"start","kind":"tunnel","id":1}}
```

| `action` | `kind` | What the owner does |
| --- | --- | --- |
| `start`, `stop`, `restart` | `tunnel` | start, stop, or stop then start. Requires `id`, or `target` on the request (a name or id, the same match as `norka connect`) |
| `save` | `tunnel`, `jumper`, `group` | `id` 0, or no id, creates. A positive `id` updates. The body is `tunnel`, `jumper`, or `group`, the same fields the window uses |
| `delete` | `tunnel`, `jumper`, `group` | deletes by `id` |

A successful `save` or `delete` returns the new `state` and emits a `config` event to subscribers. `start`, `stop`, and `restart` return `tunnels` with that one tunnel; the status change is also a `status` event on `subscribe`.

A jumper password is allowed on `save`. The keychain takes it. It is not in the snapshot or the response.

## shutdown

The owner stops its tunnels and exits.

```json
{"v":2,"token":"…","op":"shutdown"}
```

```json
{"v":2,"token":"…","op":"shutdown","force":true}
```

- A daemon stops even without `force`.
- A window (`owner` is `gui`) answers `ignored` and does not stop anything until `"force": true`.
- `norka-cli daemon stop` sends the request without `force`. `norka-cli daemon stop --force` sets `force`.

`ok` means the stop was accepted. The process releases the channel and `engine.lock` after that line.

## handover

The owner stops its tunnels and releases `engine.lock` so the caller can `Acquire`. This is not `shutdown`: the lock is released before the response, and the wait is bounded.

```json
{"v":2,"token":"…","op":"handover","timeout_ms":10000}
```

`timeout_ms` is how long to wait for the tunnels to stop. Zero or a missing field means 10 seconds. `norka-cli daemon handover` sends 10 seconds.

Success:

```json
{"v":2,"ok":true,"code":"ok","exitCode":0,"running":true,"message":"engine lock released"}
```

After that line the lock is free, and the process exits so the next owner can take the socket.

If the tunnels do not stop in time:

```json
{"v":2,"ok":false,"code":"timeout","exitCode":7,"running":true,"message":"handover timed out after 10s; the owner still holds the engine lock"}
```

The owner still holds the lock. The tunnels may still be stopping.

A window does not ignore `handover`: handing the lock to the daemon does not need `--force`. A process that is listening but does not hold the lock returns the same error as for `control`.
