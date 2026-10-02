//go:build windows

package ipc

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func currentUserSID() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	if user.User.Sid == nil {
		return "", fmt.Errorf("current user sid is empty")
	}
	return user.User.Sid.String(), nil
}
