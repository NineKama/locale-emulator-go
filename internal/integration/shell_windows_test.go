package integration

import "testing"

func TestCommandQuoting(t *testing.T) {
	got, err := Command(`C:\Apps & Games\Locale Studio.exe`)
	if err != nil || got != `"C:\Apps & Games\Locale Studio.exe" --launch "%1"` {
		t.Fatalf("command = %q, %v", got, err)
	}
	for _, bad := range []string{`relative.exe`, "C:\\bad\"name.exe", "C:\\bad\x00.exe"} {
		if _, err := Command(bad); err == nil {
			t.Errorf("accepted invalid path %q", bad)
		}
	}
}

func TestLaunchArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"game.exe"}, {"--launch"}, {"--launch", ""}, {"--launch", "game.exe", "extra"}} {
		if Target(args, `C:\Games`) != "" {
			t.Errorf("unexpected launch for %q", args)
		}
	}
	if got := Target([]string{"--launch", `folder\game.exe`}, `C:\Games`); got != `C:\Games\folder\game.exe` {
		t.Fatal(got)
	}
	path := "C:\\Games\\\u65e5\u672c\u8a9e & game.exe"
	if got := Target([]string{"--launch", path}, `D:\Other`); got != path {
		t.Fatal(got)
	}
}
