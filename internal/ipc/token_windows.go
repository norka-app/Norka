//go:build windows

package ipc

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

func fileIsUserOnly(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	if sd == nil {
		return fmt.Errorf("automation token file has no security descriptor")
	}
	text := sd.String()
	if !strings.Contains(text, ";;;"+sid+")") {
		return fmt.Errorf("automation token file is not limited to the current user")
	}
	for _, broad := range []string{"WD", "AU", "BA", "BU", "S-1-1-0", "S-1-5-11", "S-1-5-32-545", "S-1-5-32-544"} {
		if strings.Contains(text, ";;;"+broad+")") {
			return fmt.Errorf("automation token file is readable by other users")
		}
	}
	return nil
}

func hardenFile(path string) error {
	sid, err := currentUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + sid + ")")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}
