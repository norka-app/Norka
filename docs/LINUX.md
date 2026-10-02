# Norka на Linux

Сборка — Wails v2 и WebKitGTK. Готовые файлы релиза:

| Файл | Что это |
|---|---|
| `norka-x86_64.AppImage` | один файл, архитектура x86_64 |
| `norka_<версия>_amd64.deb` | пакет Debian/Ubuntu |
| `norka_<версия>_linux_amd64.tar.gz` | бинарник, иконка и `.desktop` |

Имена `norka.exe` и `norka.dmg` не меняются: их забирает встроенная проверка обновлений на Windows и macOS.

## Что нужно системе

Бинарник не включает GTK и WebKit, они берутся из дистрибутива:

- GTK 3 (`libgtk-3-0t64` или `libgtk-3-0`)
- WebKitGTK 4.1 (`libwebkit2gtk-4.1-0`)

Для AppImage на Ubuntu 24.04 ещё нужен FUSE 2 (`libfuse2t64`). Если FUSE нет, запуск такой:

```bash
chmod +x norka-x86_64.AppImage
./norka-x86_64.AppImage --appimage-extract-and-run
```

Пакет `.deb` сам объявляет зависимости. Установка:

```bash
sudo apt install ./norka_1.2.1_amd64.deb
```

Версию в имени файла замените на ту, что в релизе.

## Сборка из исходников

На Ubuntu 24.04 пакет WebKitGTK 4.0 уже убран, поэтому сборка идёт с тегом `webkit2_41`:

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
cd frontend && pnpm install && cd ..
wails build -platform linux/amd64 -tags webkit2_41
packaging/linux/package.sh
```

`package.sh` кладёт AppImage, `.deb` и tar.gz в `build/bin`. Для иконок нужен ImageMagick (`convert`). AppImage собирает `appimagetool`; если он не стартует без FUSE, скрипт пакует образ через `mksquashfs` и [type2-runtime](https://github.com/AppImage/type2-runtime).

## Окно

На Linux остаётся обычная рамка оконного менеджера. Своя строка заголовка без рамки — только Windows. Закрытие окна прячет его в трей, пока в меню трея не выбран «Выход».

## Каталог конфигурации (XDG)

Собранная программа пишет конфиг в `$XDG_CONFIG_HOME/norka/config.toml`. Если переменная не задана, это `~/.config/norka/config.toml`. Каталог `0700`, файл `0600`, как и на других системах.

Если каталога XDG ещё нет, а старый `~/.norka/config.toml` (или `config.root` рядом с ним) уже лежит, читается старый путь. Как только файл появляется в каталоге XDG, используется он.

В `wails dev` по-прежнему берётся `./config.toml` в текущем каталоге, если туда можно писать.

## Автозапуск

Включение в настройках создаёт файл `$XDG_CONFIG_HOME/autostart/norka.desktop` (обычно `~/.config/autostart/norka.desktop`). В `Exec` — полный путь к бинарнику. Выключение удаляет файл. Строки `Hidden=true` и `X-GNOME-Autostart-enabled=false` считаются выключенным автозапуском, даже если файл на месте.

В пакет `.deb` и в AppImage входит ярлык меню `norka.desktop` и иконки `hicolor` из `build/appicon.png`.

## Ссылки

«Открыть каталог конфигурации» вызывает `xdg-open`. Ссылки из интерфейса и из меню трея открывает Wails через системный браузер (`xdg-open`).

## Трей

Иконка — StatusNotifierItem библиотеки `github.com/energye/systray` (D-Bus, протокол KDE StatusNotifier / AppIndicator). Отдельная библиотека `libappindicator` не линкуется: нужен процесс, который смотрит шину.

| Среда | Что ожидать |
|---|---|
| KDE Plasma, Xfce с плагином индикаторов, Ubuntu (Ayatana) | иконка и меню DBusMenu |
| GNOME без расширения AppIndicator | иконки может не быть. Окно работает. Пакет рекомендует `libayatana-appindicator3-1`; на GNOME часто нужно расширение «AppIndicator and KStatusNotifierItem Support» |
| Нет сессионной шины D-Bus | регистрация иконки пишется в журнал и пропускается |

Ограничения текущей библиотеки:

- Левый клик открывает главное окно. Само меню показывает оболочка по свойству `Menu` (часто по правому клику или по левому — как настроена панель). Обработчик правого клика в библиотеке на Linux пустой.
- Подсказка и подпись иконки зависят от панели. Часть окружений рисует рядом слово «Norka» (`SetTitle`).
- Цветная иконка статуса — PNG 32×32. Монохромный template-режим macOS на Linux не используется.
- Если в сессии нет StatusNotifier watcher, меню трея недоступно. Закрытие окна всё равно прячет его; вернуть окно тогда можно только повторным запуском (сработает блокировка второго экземпляра и покажет уже открытое окно).

## Обновления

Проверка смотрит последний релиз `norka-app/Norka` и предлагает ссылку, не подменяя запущенный файл. Порядок такой: точное имя `norka-x86_64.AppImage`, любой другой `.AppImage`, затем `.deb`, иначе страница релиза. Кнопка «Скачать» открывает эту ссылку в браузере. Замену AppImage на месте приложение не делает.
