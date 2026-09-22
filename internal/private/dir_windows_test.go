package private

import (
	"golang.org/x/sys/windows"
	"strings"
	"testing"
)

func TestPrivateDirectoryProtectedACL(t *testing.T) {
	dir := t.TempDir()
	if err := Dir(dir); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	s := sd.String()
	if !strings.Contains(s, "D:P") {
		t.Fatalf("ACL inheritance is not protected: %s", s)
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, u.User.Sid.String()) || strings.Contains(s, ";;;WD)") || strings.Contains(s, ";;;BU)") {
		t.Fatalf("unexpected access grants: %s", s)
	}
}
