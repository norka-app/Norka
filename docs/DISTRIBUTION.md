# Публикация в winget, Scoop и Homebrew

Манифесты лежат в репозитории и **никуда сами не отправляются**. Репозитории `microsoft/winget-pkgs`, tap Homebrew и bucket Scoop из этого CI не создаются. Ниже — что сделать один раз и что происходит на каждом теге `v*`.

Имена файлов релиза, на которые ссылаются манифесты:

- Windows: `norka.exe` (переносной файл, не установщик)
- macOS: `norka.dmg` (ad-hoc, без нотаризации)

## Каждый релиз: хеши в этом репозитории

Выпуск `v*` кладёт рядом с файлами `SHA256SUMS`: строка — хеш, два пробела, имя файла. Образцам манифестов нужны две строки:

- `norka.exe` — winget (`InstallerSha256`, заглавные буквы) и Scoop (`hash`, строчные)
- `norka.dmg` — Homebrew (`sha256`, строчные)

Сейчас в `packaging/` версия `1.2.1`. Хеши взяты из `SHA256SUMS` выпуска [v1.2.1](https://github.com/norka-app/Norka/releases/tag/v1.2.1).

После публикации выпуска (не черновика) workflow `.github/workflows/update-manifests.yml` делает шаг сам:

1. Скачивает `SHA256SUMS` этого тега.
2. Переписывает версию, дату, ссылки на тег и хеши в `packaging/winget/`, `packaging/scoop/norka.json` и `packaging/homebrew/norka.rb`. В Scoop строка `v$version` внутри `autoupdate` не трогается: там нет конкретного номера.
3. Коммитит это в ветку `packaging/manifests-v<версия>` и открывает pull request токеном `GITHUB_TOKEN`.

Теги и страницы Release workflow не создаёт и не меняет. Предварительный тег с дефисом (`v1.2.0-beta.1`) пропускается: в образцы стабильных манифестов он не пишется. Ручной запуск workflow (кнопка Run workflow) принимает версию явно и эту проверку не делает.

Если в настройках репозитория выключено «Allow GitHub Actions to create and approve pull requests», Actions не откроет PR. По умолчанию эта галка выключена, и токен запуска её сам не включает. Тогда файлы обновляет скрипт, а pull request открывает человек или облачный агент:

```bash
scripts/update-manifests.sh 1.2.1
```

Версию можно передать с `v` или без. Скрипт только меняет файлы в рабочей копии: не пушит, не ставит теги и не редактирует выпуски. Дальше обычный коммит и PR.

## Один раз: winget

Идентификатор пакета: `NorkaApp.Norka`. Файлы для первой заявки: `packaging/winget/`.

1. Поставьте [wingetcreate](https://github.com/microsoft/winget-create) или проверяйте YAML по схемам из комментария в начале каждого файла.
2. Перед заявкой сверьте `PackageVersion`, `InstallerUrl` и `InstallerSha256` с тем релизом, который уже выложен. Сейчас это версия `1.2.1` и SHA256 `norka.exe` из `SHA256SUMS` тега `v1.2.1`. `InstallerSha256` записан заглавными буквами. На следующем выпуске те же поля перепишет workflow хешей или `scripts/update-manifests.sh`.
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
2. Если `WINGET_TOKEN` задан, пакет `NorkaApp.Norka` уже есть в `winget-pkgs` и в теге нет дефиса, шаг обновляет манифест и открывает PR в `winget-pkgs`. Тег вроде `v1.2.0-beta.1` — предварительный выпуск, в WinGet он не отправляется. Сборка релиза локальные файлы `packaging/winget/` не переписывает. Их обновляет workflow хешей после публикации выпуска; на автопубликацию в `winget-pkgs` это не влияет.
3. PR в `winget-pkgs` мержит модератор Microsoft, не этот репозиторий.

## Один раз: Scoop

Отдельный репозиторий, его нужно создать руками: **`norka-app/scoop-bucket`**. Из этого репозитория он не создаётся.

1. Создайте публичный репозиторий `norka-app/scoop-bucket`.
2. Положите в корень файл `bucket/norka.json` — копия `packaging/scoop/norka.json`. Каталог `bucket/` нужен, если в репозитории будут и другие манифесты; один файл `norka.json` в корне тоже допустим, тогда команда установки та же.
3. В манифесте уже стоят `checkver` по релизам GitHub и `autoupdate` на `norka.exe` версии `$version`. Хеш в блоке `autoupdate` не указан: `checkver -u` скачивает файл и считает SHA256 сам.

Пользователь после появления репозитория:

```bash
scoop bucket add norka https://github.com/norka-app/scoop-bucket
scoop install norka
```

Пока репозитория нет, команды в README помечены «скоро».

Чтобы bucket обновлялся сам, в **том** репозитории добавьте workflow (не в Norka):

```yaml
name: Update manifest
on:
  schedule:
    - cron: "17 4 * * *"
  workflow_dispatch:
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ScoopInstaller/GithubActions@main
        with:
          buckets: |
            norka-app/scoop-bucket
```

Либо на машине с Scoop: `scoop update && cd scoop-bucket && bin/checkver.ps1 -u norka` и коммит. Секрет в репозитории Norka для Scoop не нужен.

## Каждый релиз: Scoop

Новый тег `v*` выкладывает `norka.exe`. Образец `packaging/scoop/norka.json` в этом репозитории обновляет workflow хешей. Манифест в bucket подхватывает выпуск отдельно, через `checkver` / `autoupdate`, когда отработает workflow bucket или ручной `checkver -u`. Пока это не сделано, в bucket остаётся прошлая версия.

## Один раз: Homebrew

Отдельный репозиторий, его нужно создать руками: **`norka-app/homebrew-tap`**. Имя `homebrew-` в начале даёт tap `norka-app/tap`.

1. Создайте публичный репозиторий `norka-app/homebrew-tap`.
2. Положите туда `Casks/norka.rb` — копия `packaging/homebrew/norka.rb`.
3. Перед первым `brew audit` сверьте `version` и `sha256` с `norka.dmg` релиза (`shasum -a 256 norka.dmg` или строка `norka.dmg` в `SHA256SUMS`). Сейчас в cask версия `1.2.1` и SHA256 `norka.dmg` из `SHA256SUMS` выпуска v1.2.1. Дальше образец в этом репозитории обновляет тот же workflow, что и остальные манифесты.
4. В cask есть `caveats`: сборка ad-hoc, без нотаризации, macOS ставит карантин. Снять его: `xattr -dr com.apple.quarantine /Applications/norka.app`.
5. `zap trash: "~/.norka"` — каталог конфига на macOS.

Пользователь:

```bash
brew tap norka-app/tap
brew install --cask norka-app/tap/norka
```

Короткая форма после tap: `brew install --cask norka`.

Автообновление cask в **том** репозитории, например действием `Homebrew/actions` или ручным коммитом. Образец workflow для tap:

```yaml
name: Update cask
on:
  workflow_dispatch:
  repository_dispatch:
    types: [norka-release]
jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Bump cask from latest GitHub release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          tag="$(gh release view --repo norka-app/Norka --json tagName --jq .tagName)"
          ver="${tag#v}"
          gh release download "$tag" --repo norka-app/Norka --pattern norka.dmg
          sum="$(shasum -a 256 norka.dmg | awk '{print $1}')"
          ruby -i -pe "BEGIN{v=ENV.fetch('VER'); s=ENV.fetch('SUM')}; \$_.sub!(/version \".*\"/, %(version \"#{v}\")); \$_.sub!(/sha256 \".*\"/, %(sha256 \"#{s}\"))" \
            VER="$ver" SUM="$sum" Casks/norka.rb
          git diff --exit-code && exit 0
          git config user.name "github-actions"
          git config user.email "actions@github.com"
          git add Casks/norka.rb
          git commit -m "norka $ver"
          git push
```

`GITHUB_TOKEN` tap-репозитория читает публичный релиз Norka без отдельного секрета. Можно вместо расписания слать `repository_dispatch` из Norka — для этого понадобится PAT с правом `contents:write` на tap. Пока его нет, обновляйте cask вручную или расписанием в tap. В Norka такой шаг не добавлен, чтобы не открывать PR и не пушить во внешний репозиторий без явного секрета.

## Каждый релиз: Homebrew

Тег `v*` публикует `norka.dmg`. Образец `packaging/homebrew/norka.rb` обновляет workflow хешей. Cask в tap сам не меняется, пока не обновите `version` и `sha256` там (workflow tap выше или правка `Casks/norka.rb`). `livecheck` в cask смотрит последний релиз GitHub, поэтому `brew livecheck` покажет новую версию ещё до правки файла в tap.

## Что не делается из этого репозитория

- Нет PR в `microsoft/winget-pkgs`, пока не задан `WINGET_TOKEN` и пока первая версия не принята.
- Репозитории `norka-app/scoop-bucket` и `norka-app/homebrew-tap` не создаются.
- macOS не подписывается Developer ID и не нотаризуется.
- Workflow хешей и `scripts/update-manifests.sh` не создают теги и не меняют выпуски. Они правят только образцы манифестов в этом репозитории.
