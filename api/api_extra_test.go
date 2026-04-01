package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"lazypx/api"
)

// ── Error path tests for all API functions ────────────────────────────────────

func TestGetVMs_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetVMs(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVMAction_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.StartVM(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteVM_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.DeleteVM(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateVM_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.MigrateVM(context.Background(), "pve1", 100, "pve2", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBackupVM_OK(t *testing.T) {
	_, c := mockServer(t, 200, `{"data":"UPID:pve1:0000:0000:0000:vzdump:100:root@pam:"}`)
	upid, err := c.BackupVM(context.Background(), "pve1", 100, "local")
	if err != nil {
		t.Fatal(err)
	}
	if upid == "" {
		t.Fatal("expected UPID")
	}
}

func TestBackupVM_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.BackupVM(context.Background(), "pve1", 100, "local")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetContainers_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetContainers(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCTAction_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.StartCT(context.Background(), "pve1", 200)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteCT_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.DeleteCT(context.Background(), "pve1", 200)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetStorage_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetStorage(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Snapshots error paths ────────────────────────────────────────────────────

func TestGetSnapshots_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetSnapshots(context.Background(), "pve1", 100, "qemu")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSnapshots_LXC(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/lxc/200/snapshot": jsonOK([]map[string]any{
			{"name": "current"},
		}),
	})
	snaps, err := c.GetSnapshots(context.Background(), "pve1", 200, "lxc")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Name != "current" {
		t.Errorf("unexpected: %+v", snaps)
	}
}

func TestCreateSnapshot_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.CreateSnapshot(context.Background(), "pve1", 100, "qemu", "test", "desc")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteSnapshot_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.DeleteSnapshot(context.Background(), "pve1", 100, "qemu", "backup1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRollbackSnapshot_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.RollbackSnapshot(context.Background(), "pve1", 100, "qemu", "backup1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Tasks error paths ────────────────────────────────────────────────────────

func TestGetRecentTasks_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetRecentTasks(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTaskStatus_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetTaskStatus(context.Background(), "pve1", "UPID:1234")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTaskLog_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetTaskLog(context.Background(), "pve1", "UPID:1234", 0, 50)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetClusterTasks_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetClusterTasks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWatchTask_ConsecutiveErrors(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	ch := make(chan api.TaskLog, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c.WatchTask(ctx, "pve1", "UPID:1234", ch)
	for range ch {
	}
}

func TestWatchTask_ContextCancelled(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/log": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"n": 0, "t": "working"}}})
		},
		"/api2/json/nodes/pve1/tasks/UPID:1234/status": jsonOK(map[string]any{
			"status": "running",
		}),
	})
	ch := make(chan api.TaskLog, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	c.WatchTask(ctx, "pve1", "UPID:1234", ch)
	for range ch {
	}
}

func TestWatchTask_StatusError(t *testing.T) {
	logCount := 0
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/log": func(w http.ResponseWriter, r *http.Request) {
			logCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if logCount == 1 {
				json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"n": 0, "t": "line1"}}})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			}
		},
		"/api2/json/nodes/pve1/tasks/UPID:1234/status": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			w.Write([]byte(`{"errors":{"detail":"fail"}}`))
		},
	})
	ch := make(chan api.TaskLog, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c.WatchTask(ctx, "pve1", "UPID:1234", ch)
	var logs []api.TaskLog
	for l := range ch {
		logs = append(logs, l)
	}
	if len(logs) < 1 {
		t.Error("expected at least 1 log line")
	}
}

// ── Network error paths ──────────────────────────────────────────────────────

func TestGetNetworkInterfaces_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetNetworkInterfaces(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetGuestAgentNetworkInterfaces_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetGuestAgentNetworkInterfaces(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetClusterFirewallRules_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetClusterFirewallRules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetNodeFirewallRules_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetNodeFirewallRules(context.Background(), "pve1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetVMFirewallRules_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetVMFirewallRules(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Cluster error paths ──────────────────────────────────────────────────────

func TestGetClusterStatus_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetClusterStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetClusterResources_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetClusterResources(context.Background(), "vm")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPools_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetPools(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPool_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetPool(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreatePool_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.CreatePool(context.Background(), "testpool", "comment")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeletePool_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.DeletePool(context.Background(), "testpool")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Access error paths ───────────────────────────────────────────────────────

func TestGetUsers_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetUsers(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateUser_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.CreateUser(context.Background(), "test@pam", "pass", "e@e.com", "test", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteUser_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.DeleteUser(context.Background(), "test@pam")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetGroups_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetGroups(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateGroup_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.CreateGroup(context.Background(), "devops", "comment")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteGroup_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.DeleteGroup(context.Background(), "devops")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetRoles_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetRoles(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateRole_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.CreateRole(context.Background(), "MyRole", "VM.Allocate")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteRole_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.DeleteRole(context.Background(), "MyRole")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetACL_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetACL(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateACL_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	err := c.UpdateACL(context.Background(), "/", "root@pam", "user", "PVEAdmin", true, false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateACL_Group(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/acl": captureMethod(t, new(string), new(string)),
	})
	err := c.UpdateACL(context.Background(), "/", "admins", "group", "PVEAdmin", false, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateACL_Token(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/acl": captureMethod(t, new(string), new(string)),
	})
	err := c.UpdateACL(context.Background(), "/", "root@pam!mytoken", "token", "PVEAdmin", true, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetTokens_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetTokens(context.Background(), "root@pam")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── HA error paths ───────────────────────────────────────────────────────────

func TestGetHAResources_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetHAResources(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetHAGroups_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetHAGroups(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetHAStatus_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetHAStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Backups error paths ──────────────────────────────────────────────────────

func TestGetBackups_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetBackups(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateBackup_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.CreateBackup(context.Background(), "pve1", 100, "my-backup")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Volume error paths ───────────────────────────────────────────────────────

func TestGetVolumes_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetVolumes(context.Background(), "pve1", "local-lvm", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteVolume_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.DeleteVolume(context.Background(), "pve1", "local-lvm", "local-lvm:vm-100-disk-0")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResizeDisk_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.ResizeDisk(context.Background(), "pve1", 100, "qemu", "scsi0", "+10G")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResizeDisk_LXC(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/lxc/200/resize": captureMethod(t, new(string), new(string)),
	})
	_, err := c.ResizeDisk(context.Background(), "pve1", 200, "lxc", "rootfs", "+5G")
	if err != nil {
		t.Fatal(err)
	}
}

func TestMoveDisk_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.MoveDisk(context.Background(), "pve1", 100, "scsi0", "local-lvm", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── VMConfig error paths ─────────────────────────────────────────────────────

func TestGetVMConfig_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetVMConfig(context.Background(), "pve1", 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetCTConfig_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.GetCTConfig(context.Background(), "pve1", 200)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloneVM_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.CloneVM(context.Background(), "pve1", 100, 101, "clone", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloneCT_Error(t *testing.T) {
	_, c := mockServer(t, 500, `{"errors":{"detail":"server error"}}`)
	_, err := c.CloneCT(context.Background(), "pve1", 200, 201, "clone")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── do() connection refused ──────────────────────────────────────────────────

func TestDo_ConnectionRefused(t *testing.T) {
	c := api.NewClient("https://127.0.0.1:1", "root@pam!test", "secret", true)
	_, err := c.GetNodes(context.Background())
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// ── doWithRetry context cancellation during backoff ──────────────────────────

func TestDoWithRetry_ContextCancelledDuringBackoff(t *testing.T) {
	callCount := 0
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/version": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(500)
			w.Write([]byte(`{"errors":{"detail":"server error"}}`))
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err := c.Ping(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Remaining edge cases for 100% coverage ───────────────────────────────────

func TestGetBackups_SkipNonBackupStorage(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{
			{"storage": "local", "type": "dir", "active": 1, "content": "iso,vztmpl", "total": 100, "used": 0},
		}),
	})
	backups, err := c.GetBackups(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}
}

func TestGetBackups_ContentFetchError(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{
			{"storage": "backup", "type": "dir", "active": 1, "content": "backup", "total": 100, "used": 0},
		}),
		"/api2/json/nodes/pve1/storage/backup/content": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte(`{"errors":{"detail":"storage offline"}}`))
		},
	})
	backups, err := c.GetBackups(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err) // GetBackups continues on content fetch error, doesn't fail
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}
}

func TestDo_DecodeError(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/version": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`not-valid-json{{{`))
		},
	})
	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestWatchTask_ChannelSendCancelled(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/log": jsonOK([]map[string]any{
			{"n": 0, "t": "line1"},
		}),
		"/api2/json/nodes/pve1/tasks/UPID:1234/status": jsonOK(map[string]any{
			"status": "running",
		}),
	})
	ch := make(chan api.TaskLog) // unbuffered — send blocks until read
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	c.WatchTask(ctx, "pve1", "UPID:1234", ch)
	for range ch {
	}
}

func TestCloneVM_WithTarget(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/clone": captureMethod(t, &gotMethod, new(string)),
	})
	upid, err := c.CloneVM(context.Background(), "pve1", 100, 101, "clone", "pve2")
	if err != nil {
		t.Fatal(err)
	}
	if upid != "OK" {
		t.Errorf("UPID = %q", upid)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}
