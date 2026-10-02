//go:build !windows && !darwin

package uilocale

func platformLocale() string {
	return ""
}
