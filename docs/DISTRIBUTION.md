# Публикация в winget, Scoop и Homebrew

Образцы манифестов лежат в `packaging/`. Репозитории `microsoft/winget-pkgs`, `norka-app/homebrew-tap` и `norka-app/scoop-bucket` этот CI не создаёт.

На каждом теге `v*` задача `release` выкладывает файлы приложения и общий `SHA256SUMS`. Сразу после неё workflow `Publish Norka app` (`.github/workflows/publish-app.yml`) пушит cask `Casks/norka.rb` в `norka-app/homebrew-tap` и манифест `bucket/norka.json` в `norka-app/scoop-bucket`. Текст берётся из `packaging/homebrew/norka.rb` и `packaging/scoop/norka.json`. Версия и хеши берутся из `SHA256SUMS` этого выпуска, ссылки — на его `norka.dmg` и `norka.exe`. Push идёт секретом `TAP_GITHUB_TOKEN`, тем же, что у `norka-cli`. Нет секрета — push пропускается, выпуск не считается упавшим. Файлы `norka-cli`, файлы приложения и общий `SHA256SUMS` этот workflow не меняет.

winget по-прежнему не публикуется, пока нет `WINGET_TOKEN` и первой принятой версии. В README он помечен «скоро».

Ниже — что сделать один раз и что происходит на каждом теге `v*`.

Имена файлов релиза, на которые ссылаются манифесты:

- Windows: `norka.exe` (переносной файл, не установщик)
- macOS: `norka.dmg` (ad-hoc, без нотаризации)

## Каждый релиз: хеши в этом репозитории

Выпуск `v*` кладёт рядом с файлами `SHA256SUMS`: строка — хеш, два пробела, имя файла. Образцам манифестов нужны две строки:

- `norka.exe` — winget (`InstallerSha256`, заглавные буквы) и Scoop (`hash`, строчные)
- `norka.dmg` — Homebrew (`sha256`, строчные)

Номер приложения — `1.7.0`. В `packaging/` пока версия `1.6.1`, хеши взяты из `SHA256SUMS` выпуска [v1.6.1](https://github.com/norka-app/Norka/releases/tag/v1.6.1). После тега `v1.7.0` workflow хешей перепишет версию и хеши и откроет pull request. Сам выпуск файлы в `packaging/` не меняет. Cask и манифест Scoop в tap и bucket обновляет `Publish Norka app`: он читает `SHA256SUMS` выпуска и не ждёт этот pull request.

Сборка по тегу `v*` публикует выпуск из `build.yml` токеном `GITHUB_TOKEN`. Событие, которое создал этот токен, другие workflow не запускает, поэтому отдельный триггер `on: release` здесь не используется. Когда задача `release` прошла и в теге нет дефиса, `build.yml` вызывает `.github/workflows/update-manifests.yml` через `workflow_call` и передаёт версию.

Перед созданием выпуска `scripts/check-release-tag.sh` сверяет три вещи. Коммит тега входит в `origin/main`. Номер тега совпадает с `info.productVersion` в `wails.json` (из него `packaging/linux/package.sh` берёт имя `.deb` и `.tar.gz`). В коммите есть `docs/releases/<тег>.md`: этот файл и есть текст выпуска, к нему дописывается хвост про SmartScreen, если маркера в файле ещё нет. Нет хотя бы одного условия, задача `release` падает и выпуск не создаётся. Проверка лежит в workflow того коммита, на который указывает тег. Тег на старом коммите без этого шага её не запустит.

Видео на странице выпуска вставляют в тот же `docs/releases/<тег>.md`. На отдельной строке ставят ссылку вида `https://github.com/user-attachments/assets/...`: её GitHub сам показывает плеером. Файл, закоммиченный в репозиторий, на странице выпуска остаётся ссылкой на скачивание.

Этот workflow:

1. Скачивает `SHA256SUMS` этого тега.
2. Переписывает версию, дату, ссылки на тег и хеши в `packaging/winget/`, `packaging/scoop/norka.json` и `packaging/homebrew/norka.rb`. В Scoop строка `v$version` внутри `autoupdate` не трогается: там нет конкретного номера.
3. Коммитит это в ветку `packaging/manifests-v<версия>` и открывает pull request тем же `GITHUB_TOKEN`. Если токену нельзя создавать pull request, ветка всё равно отправляется, в журнал пишется ссылка на сравнение, и задача завершается успешно.

Теги и страницы Release этот шаг не создаёт и не меняет. Предварительный тег с дефисом (`v1.2.0-beta.1`) сборка в workflow не передаёт. Ручной запуск (кнопка Run workflow) принимает версию явно и эту проверку не делает.

Pull request, открытый `GITHUB_TOKEN`, сам сборки не запускает. Для правки только манифестов это нормально. Чтобы прогнать CI, закройте PR и откройте снова или отправьте в ветку ещё один коммит.

Если в настройках репозитория выключено «Allow GitHub Actions to create and approve pull requests», Actions не откроет PR. По умолчанию эта галка выключена, и токен запуска её сам не включает. Ветка `packaging/manifests-v<версия>` к этому моменту уже отправлена. Workflow печатает ссылку и завершается с кодом 0, поэтому выпуск не считается упавшим:

https://github.com/norka-app/Norka/compare/main...packaging/manifests-v<версия>

Pull request по этой ссылке открывает человек. Если ветку нужно собрать локально, те же файлы правит скрипт:

```bash
scripts/update-manifests.sh 1.7.0
```

Версию можно передать с `v` или без. Скрипт только меняет файлы в рабочей копии: не пушит, не ставит теги и не редактирует выпуски. Дальше обычный коммит и PR.

## Один раз: winget

Идентификатор пакета: `NorkaApp.Norka`. Файлы для первой заявки: `packaging/winget/`.

1. Поставьте [wingetcreate](https://github.com/microsoft/winget-create) или проверяйте YAML по схемам из комментария в начале каждого файла.
2. Перед заявкой сверьте `PackageVersion`, `InstallerUrl` и `InstallerSha256` с тем релизом, который уже выложен. Пока тег `v1.7.0` не опубликован, в образцах версия `1.6.1` и SHA256 `norka.exe` из `SHA256SUMS` тега `v1.6.1`. `InstallerSha256` записан заглавными буквами. На выпуске `v1.7.0` те же поля перепишет workflow хешей или `scripts/update-manifests.sh`.
3. Скопируйте четыре файла в свой форк [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs) по пути:

   `manifests/n/NorkaApp/Norka/<версия>/`

   Имена файлов те же: `NorkaApp.Norka.yaml`, `NorkaApp.Norka.installer.yaml`, `NorkaApp.Norka.locale.en-US.yaml`, `NorkaApp.Norka.locale.ru-RU.yaml`.
4. Откройте pull request в `microsoft/winget-pkgs` по их шаблону. Пока его не смержат, пакета в `winget` нет.
5. Форк `winget-pkgs` должен жить у владельца, от имени которого потом работает автоматизация. Для организации это пользователь или бот, совпадающий с `github.repository_owner` (`norka-app`), либо задайте в шаге `fork-user`.

Секрет для следующих релизов:

| Секрет | Что это |
|---|---|
| `WINGET_TOKEN` | GitHub PAT со scope `public_repo` у владельца форка `winget-pkgs`. Fine-grained token тоже годится, если может писать в форк и открывать PR в `microsoft/winget-pkgs` |

Пока секрета нет, шаг «Publish to WinGet» пропускается и внешний PR не создаётся.

Действие: `vedantmgoyal9/winget-releaser@v2`. Оно ищет в релизе файл по регулярному выражению `^norka\.exe$` и открывает PR с новой версией. Первой версии в `winget-pkgs` ещё нет — действие само об этом напишет и завершится ошибкой, поэтому секрет добавляйте после ручного PR.

## Каждый релиз: winget

1. Тег `v*` собирает `norka.exe` и создаёт GitHub Release.
2. Если `WINGET_TOKEN` задан, пакет `NorkaApp.Norka` уже есть в `winget-pkgs` и в теге нет дефиса, шаг обновляет манифест и открывает PR в `winget-pkgs`. Тег вроде `v1.2.0-beta.1` — предварительный выпуск, в WinGet он не отправляется. Задача `release` локальные файлы `packaging/winget/` не переписывает. На теге без дефиса после неё сборка вызывает workflow хешей; на автопубликацию в `winget-pkgs` это не влияет.
3. PR в `winget-pkgs` мержит модератор Microsoft, не этот репозиторий.

## Один раз: Scoop

Репозиторий **`norka-app/scoop-bucket`** уже есть: в нём лежит клиент. Этот CI его не создаёт. Манифест приложения — `bucket/norka.json`. Каталог `bucket/` общий с `bucket/norka-cli.json`; публикация приложения этот файл клиента не меняет.

В образце `packaging/scoop/norka.json` стоят `checkver` по релизам GitHub и `autoupdate` на `norka.exe` версии `$version`. Строку `v$version` публикация не переписывает. Хеш в блоке `autoupdate` не указан.

Пользователь:

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka/norka
```

## Каждый релиз: Scoop

После задачи `release` workflow `Publish Norka app` скачивает `SHA256SUMS` этого тега, подставляет версию, хеш `norka.exe` и ссылку `https://github.com/norka-app/Norka/releases/download/<тег>/norka.exe` в копию `packaging/scoop/norka.json` и пушит её как `bucket/norka.json`. Токен — `TAP_GITHUB_TOKEN`. Нет секрета — push пропускается, задача завершается успешно. Образец в `packaging/` по-прежнему обновляет workflow хешей отдельно. Bucket не ждёт этот pull request.

Перед push скрипт `scripts/publish-app-packages.sh` проверяет, что `norka.exe` по этой ссылке открывается. Повторный запуск с тем же тегом не делает пустой коммит, если файл уже совпадает.

## Один раз: Homebrew

Репозиторий **`norka-app/homebrew-tap`** уже есть. Имя `homebrew-` в начале даёт tap `norka-app/tap`. Cask приложения — `Casks/norka.rb`. Рядом лежит `Casks/norka-cli.rb`; публикация приложения его не меняет.

В cask есть `caveats`: сборка ad-hoc, без нотаризации, macOS ставит карантин. Снять его: `xattr -dr com.apple.quarantine /Applications/norka.app`. `zap trash: "~/.norka"` — каталог конфига на macOS. `url` указывает на `norka.dmg` выпуска через `v#{version}`.

Пользователь:

```bash
brew install --cask norka-app/tap/norka
```

Короткая форма после того, как tap уже добавлен: `brew install --cask norka`.

## Каждый релиз: Homebrew

После задачи `release` тот же workflow подставляет в копию `packaging/homebrew/norka.rb` версию и SHA256 строки `norka.dmg` из `SHA256SUMS` и пушит `Casks/norka.rb`. Ссылка в cask остаётся `https://github.com/norka-app/Norka/releases/download/v#{version}/norka.dmg`: вместе с полем `version` это файл этого выпуска. Перед push скрипт проверяет, что `norka.dmg` по адресу `https://github.com/norka-app/Norka/releases/download/<тег>/norka.dmg` открывается. `livecheck` смотрит последний релиз GitHub.

Секрет тот же `TAP_GITHUB_TOKEN`. Нет секрета — cask не пушится. Если в это же время `norka-cli` пушит свой cask, push приложения повторяется поверх нового `main` и по-прежнему меняет только `Casks/norka.rb`.

Ручной запуск, в том числе для уже стоящего тега `v1.6.1`: Actions → Publish Norka app → Run workflow. Ветку оставьте `main`: шаги и образцы `packaging/` берутся из неё. В поле tag укажите тег, например `v1.6.1`. Тег не переставляется. Файлы выпуска и `SHA256SUMS` не заменяются.

## norka-cli

Отдельный клиент собирает GoReleaser (`.goreleaser.yaml`) в workflow `Release norka-cli` (`.github/workflows/release-cli.yml`). На теге `v*` его вызывает задача `release-cli` в `build.yml`, после `release`. Файлы дописываются в уже созданный выпуск (`release.mode: keep-existing`). Повторный запуск заменяет файлы клиента с теми же именами (`release.replace_existing_artifacts`). Общий `SHA256SUMS` эта задача не создаёт и не перезаписывает: его по-прежнему пишет `release`, а `update-manifests` и автообновление читают только его. `norka.exe`, `norka.dmg`, AppImage, deb и tar.gz приложения тоже не трогаются. Хеши клиента лежат в `norka-cli_SHA256SUMS`. После выкладки workflow аттестует архивы, zip, deb и rpm клиента.

Платформы: linux, windows и darwin, amd64 и arm64. Архивы `tar.gz`, для Windows `zip`. Плюс пакеты deb и rpm. В каждом архиве и пакете два бинарника: `norka-cli` и `norkad`.

Публикация cask и манифеста Scoop клиента идёт в чужие репозитории и требует секрет `TAP_GITHUB_TOKEN` (право писать в `norka-app/homebrew-tap` и `norka-app/scoop-bucket`). В конфиге токен записан только как `{{ .Env.TAP_GITHUB_TOKEN }}`: другую подстановку GoReleaser отклоняет при публикации. Нет секрета — в Actions приходит пустая строка, `skip_upload` пропускает tap и bucket, а сам выпуск с архивами и пакетами собирается. Отсутствующая переменная для `skip_upload` тоже означает пропуск: шаблон смотрит через `index`, а не через `.Env.ИМЯ`. Тот же секрет использует `Publish Norka app` для `Casks/norka.rb` и `bucket/norka.json`. GoReleaser клиента эти два файла не пишет.

### Ручной перезапуск

Если публикация клиента на уже стоящем теге упала, тег не переставляют. После правки в `main` откройте Actions → Release norka-cli → Run workflow. Ветку оставьте `main`: шаги и `.goreleaser.yaml` берутся из неё. В поле tag укажите тег, например `v1.5.0`: с него берутся только исходники. Уже загруженные архивы, пакеты и `norka-cli_SHA256SUMS` заменяются новыми. Аттестация и хеши в cask и Scoop считаются от этой же сборки. Файлы GUI и общий `SHA256SUMS` остаются как были.

- Cask `norka-cli` в `norka-app/homebrew-tap`, каталог `Casks`. Cask ставит `norka-cli` и `norkad` и снимает `com.apple.quarantine` с обоих. Секция `brews` не используется.
- Манифест Scoop `bucket/norka-cli.json` в `norka-app/scoop-bucket`. В `bin` два shim: `norka-cli.exe` и `norkad.exe`, потому что оба файла лежат в том же zip.

Пользователь, когда tap и bucket уже обновлены этим выпуском:

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

deb и rpm ставятся из файлов выпуска, например `sudo dpkg -i norka-cli_*_linux_amd64.deb`.

Клиент управляет туннелями только у запущенной Norka. В **Настройки → Функции** должно быть включено «Автоматизация». Иначе команда завершается с кодом 1. `norka-cli connect` может сам запустить приложение в трее, если Norka хотя бы раз записывала `app-path` рядом с `config.toml`.

Репозитории tap и bucket этот workflow не создаёт. Они уже есть; `norka-cli` только дописывает в них `Casks/norka-cli.rb` и `bucket/norka-cli.json`.

## Что не делается из этого репозитория

- Нет PR в `microsoft/winget-pkgs`, пока не задан `WINGET_TOKEN` и пока первая версия не принята.
- Репозитории `norka-app/scoop-bucket` и `norka-app/homebrew-tap` не создаются.
- macOS не подписывается Developer ID и не нотаризуется.
- Workflow хешей и `scripts/update-manifests.sh` не создают теги и не меняют выпуски. Они правят только образцы манифестов в этом репозитории.
- `Publish Norka app` и `scripts/publish-app-packages.sh` не создают теги и не меняют выпуски, общий `SHA256SUMS` и файлы `norka-cli`. Они пушат только `Casks/norka.rb` и `bucket/norka.json`, и только если задан `TAP_GITHUB_TOKEN`.
