package automation

import "strings"

// Message is a short sentence for the CLI and for JSON responses.
// locale is ru or en. name is a tunnel name. detail is an error from the
// tunnel runtime. names lists ambiguous matches.
func Message(locale, code, name, detail string, names []string) string {
	ru := strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "ru")
	list := strings.Join(names, ", ")
	switch code {
	case "disabled":
		if ru {
			return "Автоматизация выключена. Включите её в Настройки → Функции."
		}
		return "Automation is off. Turn it on in Settings → Features."
	case "not_running":
		if ru {
			return "Norka не запущена."
		}
		return "Norka is not running."
	case "not_found":
		if ru {
			return "Туннель «" + name + "» не найден."
		}
		return "Tunnel “" + name + "” was not found."
	case "ambiguous":
		if ru {
			return "Имя «" + name + "» подходит нескольким туннелям: " + list + ". Укажите имя целиком или id."
		}
		return "“" + name + "” matches more than one tunnel: " + list + ". Use the full name or the id."
	case "connected":
		if ru {
			return "Туннель «" + name + "» подключён."
		}
		return "Tunnel “" + name + "” is connected."
	case "disconnected":
		if ru {
			return "Туннель «" + name + "» отключён."
		}
		return "Tunnel “" + name + "” is disconnected."
	case "failed":
		if detail == "" {
			detail = "unknown error"
		}
		if ru {
			return "Не удалось выполнить команду для «" + name + "»: " + detail
		}
		return "The command failed for “" + name + "”: " + detail
	case "unauthorized":
		if ru {
			return "Доступ к локальному каналу Norka отклонён."
		}
		return "Access to the local Norka channel was denied."
	case "invalid":
		if ru {
			return "Ссылка norka:// не распознана."
		}
		return "This norka:// link was not recognised."
	case "bad_request":
		if ru {
			return "Неверная команда."
		}
		return "Invalid command."
	default:
		if detail != "" {
			return detail
		}
		return code
	}
}

// StatusLine is the first line of `norka status`.
func StatusLine(locale string, total, running int) string {
	ru := strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "ru")
	if total == 0 {
		if ru {
			return "Norka запущена. Туннелей нет."
		}
		return "Norka is running. There are no tunnels."
	}
	if ru {
		return "Norka запущена. Туннелей: " + itoa(total) + ", подключено: " + itoa(running) + "."
	}
	return "Norka is running. Tunnels: " + itoa(total) + ", connected: " + itoa(running) + "."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
