package core

import "fmt"

func RequireNonRoot(uid int) error {
	if uid == 0 {
		return fmt.Errorf("refusing to run Bellum installer or uninstaller as root")
	}
	return nil
}
