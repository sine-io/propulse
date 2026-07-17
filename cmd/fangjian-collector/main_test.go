package main

import "testing"

func TestRunRequiresAllCredentialEnvironmentVariablesBeforeNetworkAccess(t *testing.T) {
	for _, name := range []string{"FANGJIAN_AUTHORIZATION", "FANGJIAN_AK", "FANGJIAN_VERSION"} {
		t.Setenv(name, "")
	}
	if status := run([]string{"--community", "all"}); status != 1 {
		t.Fatalf("run() status = %d, want 1", status)
	}
}

func TestRunRejectsUnknownCommunityBeforeCollection(t *testing.T) {
	t.Setenv("FANGJIAN_AUTHORIZATION", "authorization")
	t.Setenv("FANGJIAN_AK", "ak")
	t.Setenv("FANGJIAN_VERSION", "version")
	if status := run([]string{"--community", "unknown"}); status != 2 {
		t.Fatalf("run() status = %d, want 2", status)
	}
}
