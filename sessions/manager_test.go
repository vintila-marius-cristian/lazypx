package sessions

import (
	"testing"
)

func TestManager_SessionKey(t *testing.T) {
	mgr := New("prod-cluster")
	key := mgr.SessionKey(101)
	expected := "lazypx-prod-cluster-101"
	if key != expected {
		t.Errorf("Expected %q, got %q", expected, key)
	}
}

func TestManager_SessionKey_Sanitize(t *testing.T) {
	mgr := New("my@weird!profile#name")
	key := mgr.SessionKey(999)
	expected := "lazypx-my-weird-profile-name-999"
	if key != expected {
		t.Errorf("Expected sanitized key %q, got %q", expected, key)
	}
}

func TestManager_SessionKey_DifferentVMs(t *testing.T) {
	mgr := New("default")
	k1 := mgr.SessionKey(105)
	k2 := mgr.SessionKey(150)
	if k1 == k2 {
		t.Errorf("Different VMIDs should produce different session keys: both = %q", k1)
	}
}

func TestManager_SessionKey_SameVMSameKey(t *testing.T) {
	mgr := New("default")
	k1 := mgr.SessionKey(105)
	k2 := mgr.SessionKey(105)
	if k1 != k2 {
		t.Errorf("Same VMID should always return the same key: %q vs %q", k1, k2)
	}
}

func TestManager_AttachCmd_ReturnsAttacher(t *testing.T) {
	mgr := New("default")
	key := mgr.SessionKey(105)
	attacher := mgr.AttachCmd(key)
	if attacher.key != key {
		t.Errorf("Expected attacher key %q, got %q", key, attacher.key)
	}
	if attacher.mgr != mgr {
		t.Error("Expected attacher to reference the same manager")
	}
}

func TestManager_ListSessions_Empty(t *testing.T) {
	mgr := New("default")
	sessions := mgr.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected empty session list, got %d sessions", len(sessions))
	}
}
