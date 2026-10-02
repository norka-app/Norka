# Norka

<div align="center">

**Менеджер SSH-туннелей для macOS и Windows**

Настольное приложение: jump host, проброс портов, простой компактный режим и иконка в трее. Интерфейс на русском языке.

![Version](https://img.shields.io/badge/version-0.0.1-blue)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows-blue)
![License](https://img.shields.io/badge/license-Apache%202.0-green)
![Built with Wails](https://img.shields.io/badge/built%20with-Wails%20v2.16-informational)

</div>

---

## Что это

Norka создаёт и держит SSH-туннели из окна, без ручных команд на каждый запуск. Туннели ходят через один или несколько jump host. Если связь рвётся, приложение само переподключается и показывает статус в окне и в трее.

Сборки публикуются в [Releases](../../releases): `.dmg` для macOS (universal) и `.exe` для Windows (amd64). Workflow запускает публикацию по тегу `v*`.

---

## Возможности

- **Два режима окна.** Расширенный — обзор, jump host, туннели, журнал и настройки. Простой — компактное окно на один туннель: статус, адрес, время работы, подключение и отключение. Простое окно можно закрепить поверх остальных.
- **Трей.** Иконка меняется по общему статусу туннелей. Из меню можно включить или остановить туннель, скопировать локальный адрес, открыть его в браузере, остановить все или повторить упавшие.
- **Jump host.** Пароль, файл ключа или SSH Agent, keepalive, таймаут, проверка подключения до сохранения. Цепочка из нескольких jump host на один туннель.
- **Туннели.** Local, remote и dynamic SOCKS5. Группы, поиск, копирование туннеля, автозапуск вместе с приложением, проверка доступности из диалога создания.
- **Импорт.** Туннели из команды `ssh` (`-L`, `-R`, `-D`). Jump host из файла SSH config, включая `ProxyJump`.
- **Переподключение.** После обрыва экспоненциальная пауза от 0,5 с до 1 минуты, попытки до 15 минут. В списке это статус «Переподключение».
- **Задержка.** У работающего туннеля в таблице видна задержка SSH.
- **Трафик.** Скорость и график в боковой панели, переключатель в настройках.
- **Журнал** операций с фильтром по уровню.
- **Настройки.** Светлая и тёмная тема, запуск при входе в систему, экспорт и импорт `config.toml`, свой каталог конфигурации.

---

## Режимы туннеля

| Режим | Поле `mode` | Что делает |
|------|-------------|------------|
| Local Forward | `local` | Локальный порт открывается на удалённый адрес через SSH |
| Remote Forward | `remote` | Порт на SSH-сервере открывается на локальный адрес |
| Dynamic SOCKS5 | `dynamic` | SSH-сервер работает как SOCKS5-прокси |

---

## Сборка из исходников

Нужны:

- [Go](https://go.dev/dl/) 1.25 или новее (в CI стоит 1.26)
- [Node.js](https://nodejs.org/) 18 или новее и [pnpm](https://pnpm.io/) 10 (в CI — Node.js 24)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.16

```bash
git clone <url-репозитория>
cd ssh-client

cd frontend && pnpm install && cd ..

wails dev
wails build
```

`wails build` без `-platform` собирает программу для текущей системы. В CI: `darwin/universal` и `windows/amd64`.

### Первый запуск на macOS

Сборка подписывается ad-hoc, без сертификата разработчика. Если система блокирует приложение:

1. В окне предупреждения нажмите «Отменить».
2. Откройте «Системные настройки» → «Конфиденциальность и безопасность».
3. Нажмите «Всё равно открыть».

Либо в Терминале:

```bash
sudo xattr -rd com.apple.quarantine /Applications/norka.app
```

Путь замените на тот, куда установлен `.app`.

---

## Конфигурация

Файл — TOML. Каталог и права: каталог `0700`, файл `0600`.

Куда пишется конфиг, если каталог не переопределён:

- **`wails dev`** (задана переменная `devserver`): `./config.toml`, если текущий каталог доступен на запись. Иначе `~/.norka/config.toml`.
- **Собранная программа**, в том числе автозапуск при входе: `~/.norka/config.toml`.

Если файла нет или он пустой, Norka создаёт его с пустыми списками.

Свой каталог задаётся в настройках. Рядом с исходным `config.toml` появляется файл `config.root` с абсолютным путём. Туда копируются `config.toml` и `ui.locale`. Если в целевой папке файлы уже есть, можно перезаписать их или оставить. После смены каталога приложение закрывается.

Пример:

```toml
version = 1
auto_run = false
traffic_monitor_enabled = true

[[jumpers]]
id = 1
name = "my-server"
host = "example.com"
port = 22
user = "ubuntu"
auth_type = "ssh_agent"   # password | ssh_key | ssh_agent
# key_path = "~/.ssh/id_rsa"

[[groups]]
id = 1
name = "Базы"

[[tunnels]]
id = 1
name = "local-db"
group_id = 1
jumper_ids = [1]
mode = "local"             # local | remote | dynamic
local_host = "127.0.0.1"
local_port = 5432
remote_host = "127.0.0.1"
remote_port = 5432
auto_start = true
status = "stopped"
```

Для `dynamic` поля `remote_host` и `remote_port` не нужны. Пароль jump host хранится в этом файле.

---

## Стек

| Слой | Технология |
|------|------------|
| Оболочка | [Wails](https://wails.io/) v2.16 |
| Бэкенд | Go 1.25 |
| Интерфейс | Vue 3, Vite, Naive UI |
| SSH | `golang.org/x/crypto/ssh` |
| Трей | `github.com/energye/systray` |
| Конфиг | TOML (`github.com/BurntSushi/toml`) |

---

## Лицензия

Apache MIT. Текст — в файле [LICENSE](LICENSE).
