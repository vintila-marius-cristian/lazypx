package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lazypx/api"
)

// ── Routing mock server ──────────────────────────────────────────────────────

// routeHandler maps API paths to handler functions.
type routeHandler struct {
	routes map[string]http.HandlerFunc
}

func (rh *routeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for path, fn := range rh.routes {
		if r.URL.Path == path {
			fn(w, r)
			return
		}
	}
	// Fallback: empty list
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(`{"data":[]}`))
}

func newRoutingServer(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *api.Client) {
	t.Helper()
	rh := &routeHandler{routes: routes}
	srv := httptest.NewTLSServer(rh)
	t.Cleanup(srv.Close)
	c := api.NewClient(srv.URL, "root@pam!test", "fake-secret", true)
	return srv, c
}

func jsonOK(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{"data": v})
	}
}

func jsonUPID() http.HandlerFunc {
	return jsonOK("UPID:pve1:0000:0000:0000:task:100:root@pam:")
}

func captureMethod(t *testing.T, method *string, path *string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*method = r.Method
		*path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":"OK"}`))
	}
}

// ── Nodes ─────────────────────────────────────────────────────────────────────

func TestGetNodes_Empty(t *testing.T) {
	_, c := newRoutingServer(t, nil)
	nodes, err := c.GetNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestGetNodes_WithData(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes": jsonOK([]map[string]any{
			{"node": "pve1", "status": "online", "cpu": 0.5, "maxcpu": 8, "mem": 1073741824, "maxmem": 8589934592},
			{"node": "pve2", "status": "online", "cpu": 0.25, "maxcpu": 16, "mem": 2147483648, "maxmem": 17179869184},
		}),
	})
	nodes, err := c.GetNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Node != "pve1" {
		t.Errorf("node[0].Node = %q", nodes[0].Node)
	}
	if nodes[0].MaxCPU != 8 {
		t.Errorf("node[0].MaxCPU = %d", nodes[0].MaxCPU)
	}
	if nodes[1].Node != "pve2" {
		t.Errorf("node[1].Node = %q", nodes[1].Node)
	}
}

func TestGetNodeStatus(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/status": jsonOK(map[string]any{
			"node": "pve1", "status": "online", "cpu": 0.1, "uptime": 86400,
			"kversion": "5.15.0", "pveversion": "7.4-3",
		}),
	})
	st, err := c.GetNodeStatus(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Node != "pve1" {
		t.Errorf("Node = %q", st.Node)
	}
	if st.Uptime != 86400 {
		t.Errorf("Uptime = %d", st.Uptime)
	}
	if st.Extended == nil {
		t.Fatal("Extended should not be nil")
	}
	if st.Extended.PVEVersion != "7.4-3" {
		t.Errorf("PVEVersion = %q", st.Extended.PVEVersion)
	}
}

// ── VMs ───────────────────────────────────────────────────────────────────────

func TestGetVMs(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu": jsonOK([]map[string]any{
			{"vmid": 100, "name": "web", "status": "running", "cpu": 0.3, "maxcpu": 2, "mem": 536870912, "maxmem": 2147483648},
		}),
	})
	vms, err := c.GetVMs(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if vms[0].VMID != 100 {
		t.Errorf("VMID = %d", vms[0].VMID)
	}
	if vms[0].Name != "web" {
		t.Errorf("Name = %q", vms[0].Name)
	}
}

func TestVMActions(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(*api.Client) (string, error)
		path   string
		method string
	}{
		{"StartVM", func(c *api.Client) (string, error) { return c.StartVM(context.Background(), "pve1", 100) }, "/api2/json/nodes/pve1/qemu/100/status/start", "POST"},
		{"StopVM", func(c *api.Client) (string, error) { return c.StopVM(context.Background(), "pve1", 100) }, "/api2/json/nodes/pve1/qemu/100/status/stop", "POST"},
		{"ShutdownVM", func(c *api.Client) (string, error) { return c.ShutdownVM(context.Background(), "pve1", 100) }, "/api2/json/nodes/pve1/qemu/100/status/shutdown", "POST"},
		{"RebootVM", func(c *api.Client) (string, error) { return c.RebootVM(context.Background(), "pve1", 100) }, "/api2/json/nodes/pve1/qemu/100/status/reboot", "POST"},
		{"DeleteVM", func(c *api.Client) (string, error) { return c.DeleteVM(context.Background(), "pve1", 100) }, "/api2/json/nodes/pve1/qemu/100", "DELETE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			_, c := newRoutingServer(t, map[string]http.HandlerFunc{
				tt.path: captureMethod(t, &gotMethod, &gotPath),
			})
			_, err := tt.fn(c)
			if err != nil {
				t.Fatal(err)
			}
			if gotMethod != tt.method {
				t.Errorf("method = %q, want %q", gotMethod, tt.method)
			}
			if gotPath != tt.path {
				t.Errorf("path = %q, want %q", gotPath, tt.path)
			}
		})
	}
}

func TestMigrateVM(t *testing.T) {
	var gotMethod, gotPath string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/migrate": captureMethod(t, &gotMethod, &gotPath),
	})
	upid, err := c.MigrateVM(context.Background(), "pve1", 100, "pve2", true)
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

// ── Containers ────────────────────────────────────────────────────────────────

func TestGetContainers(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/lxc": jsonOK([]map[string]any{
			{"vmid": 200, "name": "dns", "status": "running", "cpu": 0.05, "maxcpu": 1},
		}),
	})
	cts, err := c.GetContainers(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cts) != 1 || cts[0].VMID != 200 {
		t.Errorf("unexpected containers: %+v", cts)
	}
}

func TestContainerActions(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*api.Client) (string, error)
		path string
	}{
		{"StartCT", func(c *api.Client) (string, error) { return c.StartCT(context.Background(), "pve1", 200) }, "/api2/json/nodes/pve1/lxc/200/status/start"},
		{"StopCT", func(c *api.Client) (string, error) { return c.StopCT(context.Background(), "pve1", 200) }, "/api2/json/nodes/pve1/lxc/200/status/stop"},
		{"ShutdownCT", func(c *api.Client) (string, error) { return c.ShutdownCT(context.Background(), "pve1", 200) }, "/api2/json/nodes/pve1/lxc/200/status/shutdown"},
		{"RebootCT", func(c *api.Client) (string, error) { return c.RebootCT(context.Background(), "pve1", 200) }, "/api2/json/nodes/pve1/lxc/200/status/reboot"},
		{"DeleteCT", func(c *api.Client) (string, error) { return c.DeleteCT(context.Background(), "pve1", 200) }, "/api2/json/nodes/pve1/lxc/200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			_, c := newRoutingServer(t, map[string]http.HandlerFunc{
				tt.path: captureMethod(t, new(string), &gotPath),
			})
			_, err := tt.fn(c)
			if err != nil {
				t.Fatal(err)
			}
			if gotPath != tt.path {
				t.Errorf("path = %q, want %q", gotPath, tt.path)
			}
		})
	}
}

// ── Storage ───────────────────────────────────────────────────────────────────

func TestGetStorage(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{
			{"storage": "local-lvm", "type": "lvmthin", "active": 1, "total": 107374182400, "used": 53687091200, "avail": 53687091200, "content": "images,rootdir"},
		}),
	})
	st, err := c.GetStorage(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].Storage != "local-lvm" {
		t.Errorf("unexpected storage: %+v", st)
	}
}

// ── Snapshots ─────────────────────────────────────────────────────────────────

func TestGetSnapshots(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/snapshot": jsonOK([]map[string]any{
			{"name": "current", "description": "Current state"},
			{"name": "backup1", "description": "Before update", "snaptime": 1700000000},
		}),
	})
	snaps, err := c.GetSnapshots(context.Background(), "pve1", 100, "qemu")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}
	if snaps[0].Name != "current" {
		t.Errorf("snap[0].Name = %q", snaps[0].Name)
	}
	if snaps[1].Name != "backup1" {
		t.Errorf("snap[1].Name = %q", snaps[1].Name)
	}
}

func TestCreateSnapshot(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/snapshot": captureMethod(t, &gotMethod, new(string)),
	})
	upid, err := c.CreateSnapshot(context.Background(), "pve1", 100, "qemu", "test", "desc")
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

func TestDeleteSnapshot(t *testing.T) {
	var gotPath string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/snapshot/backup1": captureMethod(t, new(string), &gotPath),
	})
	_, err := c.DeleteSnapshot(context.Background(), "pve1", 100, "qemu", "backup1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "backup1") {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRollbackSnapshot(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/snapshot/backup1/rollback": captureMethod(t, &gotMethod, new(string)),
	})
	_, err := c.RollbackSnapshot(context.Background(), "pve1", 100, "qemu", "backup1")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}

// ── Tasks ─────────────────────────────────────────────────────────────────────

func TestGetRecentTasks(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks": jsonOK([]map[string]any{
			{"upid": "UPID:pve1:0000:0000::qmstart:100:root@pam:", "type": "qmstart", "status": "stopped", "exitstatus": "OK"},
		}),
	})
	tasks, err := c.GetRecentTasks(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Type != "qmstart" {
		t.Errorf("unexpected tasks: %+v", tasks)
	}
}

func TestGetClusterTasks(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/tasks": jsonOK([]map[string]any{
			{"upid": "UPID:pve1:1234", "type": "vzdump", "status": "running"},
		}),
	})
	tasks, err := c.GetClusterTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestGetTaskStatus(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/status": jsonOK(map[string]any{
			"status": "running", "upid": "UPID:1234",
		}),
	})
	ts, err := c.GetTaskStatus(context.Background(), "pve1", "UPID:1234")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Status != "running" {
		t.Errorf("Status = %q", ts.Status)
	}
}

func TestGetTaskLog(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/log": jsonOK([]map[string]any{
			{"n": 0, "t": "line 1"},
			{"n": 1, "t": "line 2"},
		}),
	})
	logs, err := c.GetTaskLog(context.Background(), "pve1", "UPID:1234", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].T != "line 1" {
		t.Errorf("log[0].T = %q", logs[0].T)
	}
}

func TestWatchTask_CompletesNormally(t *testing.T) {
	callCount := 0
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/tasks/UPID:1234/log": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			if callCount == 1 {
				json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"n": 0, "t": "working"}}})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			}
		},
		"/api2/json/nodes/pve1/tasks/UPID:1234/status": jsonOK(map[string]any{
			"status": "stopped", "exitstatus": "OK",
		}),
	})

	ch := make(chan api.TaskLog, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// ── Network ───────────────────────────────────────────────────────────────────

func TestGetNetworkInterfaces(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/network": jsonOK([]map[string]any{
			{"iface": "vmbr0", "type": "bridge", "active": 1, "address": "192.168.1.10", "netmask": "255.255.255.0", "gateway": "192.168.1.1", "bridge_ports": "eth0"},
		}),
	})
	nics, err := c.GetNetworkInterfaces(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(nics) != 1 || nics[0].Iface != "vmbr0" {
		t.Errorf("unexpected network: %+v", nics)
	}
	if nics[0].Gateway != "192.168.1.1" {
		t.Errorf("Gateway = %q", nics[0].Gateway)
	}
}

func TestGetGuestAgentNetworkInterfaces(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/agent/network-get-interfaces": jsonOK(map[string]any{
			"result": []map[string]any{
				{"name": "eth0", "ip-addresses": []map[string]any{{"ip-address": "10.0.0.5", "prefix": 24, "ip-address-type": "ipv4"}}},
			},
		}),
	})
	nets, err := c.GetGuestAgentNetworkInterfaces(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 || nets[0].Name != "eth0" {
		t.Errorf("unexpected guest nets: %+v", nets)
	}
	if len(nets[0].IPAddresses) != 1 || nets[0].IPAddresses[0].IPAddress != "10.0.0.5" {
		t.Errorf("unexpected IPs: %+v", nets[0].IPAddresses)
	}
}

// ── Cluster ───────────────────────────────────────────────────────────────────

func TestGetClusterStatus(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/status": jsonOK([]map[string]any{
			{"name": "pve1", "type": "node", "id": "node/pve1", "online": 1, "ip": "192.168.1.10"},
		}),
	})
	members, err := c.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Name != "pve1" {
		t.Errorf("unexpected members: %+v", members)
	}
}

func TestGetClusterResources(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/resources": jsonOK([]map[string]any{
			{"type": "qemu", "vmid": 100, "name": "web", "node": "pve1", "status": "running"},
		}),
	})
	res, err := c.GetClusterResources(context.Background(), "vm")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].VMID != 100 {
		t.Errorf("unexpected resources: %+v", res)
	}
}

func TestPoolCRUD(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/pools":      jsonOK([]map[string]any{{"poolid": "test", "comment": "test pool"}}),
		"/api2/json/pools/test": jsonOK(map[string]any{"poolid": "test", "comment": "test pool"}),
	})
	pools, err := c.GetPools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	pool, err := c.GetPool(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if pool.PoolID != "test" {
		t.Errorf("PoolID = %q", pool.PoolID)
	}
}

// ── Access ────────────────────────────────────────────────────────────────────

func TestGetUsers(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/users": jsonOK([]map[string]any{
			{"userid": "root@pam", "enable": 1, "firstname": "Admin"},
		}),
	})
	users, err := c.GetUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].UserID != "root@pam" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestCreateUser(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/users": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.CreateUser(context.Background(), "test@pam", "pass", "e@e.com", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestDeleteUser(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/users/test@pam": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.DeleteUser(context.Background(), "test@pam")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestGetGroups(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/groups": jsonOK([]map[string]any{{"groupid": "admins", "comment": "Administrators"}}),
	})
	groups, err := c.GetGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].GroupID != "admins" {
		t.Errorf("unexpected groups: %+v", groups)
	}
}

func TestGetRoles(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/roles": jsonOK([]map[string]any{{"roleid": "PVEAdmin", "privs": "Sys.PowerMgmt,VM.Allocate"}}),
	})
	roles, err := c.GetRoles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleID != "PVEAdmin" {
		t.Errorf("unexpected roles: %+v", roles)
	}
}

func TestGetACL(t *testing.T) {
	// Proxmox ACL endpoint returns an array directly
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/acl": jsonOK([]map[string]any{
			{"path": "/", "type": "user", "ugid": "root@pam", "roleid": "PVEAdmin", "propagate": 1},
		}),
	})
	acl, err := c.GetACL(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(acl) != 1 || acl[0].RoleID != "PVEAdmin" {
		t.Errorf("unexpected ACL: %+v", acl)
	}
}

func TestGetTokens(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/users/root@pam/token": jsonOK([]map[string]any{
			{"tokenid": "lazypx", "comment": "CLI token", "expire": 0, "privsep": 0},
		}),
	})
	tokens, err := c.GetTokens(context.Background(), "root@pam")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].TokenID != "lazypx" {
		t.Errorf("unexpected tokens: %+v", tokens)
	}
}

// ── VM Config ─────────────────────────────────────────────────────────────────

func TestGetVMConfig(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/config": jsonOK(map[string]any{
			"name": "web", "cores": 2, "memory": 2048, "scsi0": "local-lvm:vm-100-disk-0,size=32G", "net0": "virtio=BC:24:11:3B:92:A1,bridge=vmbr0",
		}),
	})
	cfg, err := c.GetVMConfig(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "web" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.Cores != 2 {
		t.Errorf("Cores = %d", cfg.Cores)
	}
	if cfg.Memory != 2048 {
		t.Errorf("Memory = %d", cfg.Memory)
	}
}

func TestGetCTConfig(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/lxc/200/config": jsonOK(map[string]any{
			"hostname": "dns", "cores": 1, "memory": 512,
		}),
	})
	cfg, err := c.GetCTConfig(context.Background(), "pve1", 200)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hostname != "dns" {
		t.Errorf("Hostname = %q", cfg.Hostname)
	}
}

func TestCloneVM(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/clone": captureMethod(t, &gotMethod, new(string)),
	})
	upid, err := c.CloneVM(context.Background(), "pve1", 100, 101, "web-clone", "")
	if err != nil {
		t.Fatal(err)
	}
	if upid != "OK" {
		t.Errorf("UPID = %q", upid)
	}
}

// ── Volumes ───────────────────────────────────────────────────────────────────

func TestGetVolumes(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage/local-lvm/content": jsonOK([]map[string]any{
			{"volid": "local-lvm:vm-100-disk-0", "format": "raw", "size": 34359738368, "vmid": "100"},
		}),
	})
	vols, err := c.GetVolumes(context.Background(), "pve1", "local-lvm", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 1 || vols[0].Format != "raw" {
		t.Errorf("unexpected volumes: %+v", vols)
	}
}

func TestDeleteVolume(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage/local-lvm/content/local-lvm:vm-100-disk-0": captureMethod(t, &gotMethod, new(string)),
	})
	_, err := c.DeleteVolume(context.Background(), "pve1", "local-lvm", "local-lvm:vm-100-disk-0")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestResizeDisk(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/resize": captureMethod(t, &gotMethod, new(string)),
	})
	_, err := c.ResizeDisk(context.Background(), "pve1", 100, "qemu", "scsi0", "+10G")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "PUT" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestMoveDisk(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/move_disk": captureMethod(t, &gotMethod, new(string)),
	})
	_, err := c.MoveDisk(context.Background(), "pve1", 100, "scsi0", "local-lvm", true)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}

// ── HA ────────────────────────────────────────────────────────────────────────

func TestGetHAResources(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/ha/resources": jsonOK([]map[string]any{
			{"sid": "vm:100", "type": "vm", "state": "started", "max_restart": 3},
		}),
	})
	res, err := c.GetHAResources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].SID != "vm:100" {
		t.Errorf("unexpected HA resources: %+v", res)
	}
}

func TestGetHAGroups(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/ha/groups": jsonOK([]map[string]any{
			{"group": "production", "nodes": "pve1,pve2"},
		}),
	})
	groups, err := c.GetHAGroups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Group != "production" {
		t.Errorf("unexpected HA groups: %+v", groups)
	}
}

func TestGetHAStatus(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/ha/status/current": jsonOK([]map[string]any{
			{"id": "vm:100", "state": "started", "node": "pve1"},
		}),
	})
	status, err := c.GetHAStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 {
		t.Fatalf("expected 1, got %d", len(status))
	}
}

// ── Backups ───────────────────────────────────────────────────────────────────

func TestGetBackups(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{
			{"storage": "backup-nfs", "type": "nfs", "active": 1, "content": "backup,iso", "total": 1099511627776, "used": 53687091200},
		}),
		"/api2/json/nodes/pve1/storage/backup-nfs/content": func(w http.ResponseWriter, r *http.Request) {
			// Verify query params
			if r.URL.Query().Get("content") != "backup" {
				t.Errorf("content = %q", r.URL.Query().Get("content"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"volid": "backup-nfs:backup/vzdump-qemu-100-2024.vma.zst", "size": 1073741824, "ctime": 1700000000},
				},
			})
		},
	})
	backups, err := c.GetBackups(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if !strings.Contains(backups[0].VolID, "vzdump") {
		t.Errorf("VolID = %q", backups[0].VolID)
	}
}

// ── Error handling ────────────────────────────────────────────────────────────

func TestProxmoxError_Error(t *testing.T) {
	e := &api.ProxmoxError{StatusCode: 500, Message: "internal"}
	if !strings.Contains(e.Error(), "500") {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestClassifyError_QuorumLoss(t *testing.T) {
	pe := &api.ProxmoxError{StatusCode: 500, Message: "cluster has lost quorum"}
	classified := api.ClassifyError(pe)
	if !errors.Is(classified, api.ErrQuorumLoss) {
		t.Errorf("expected ErrQuorumLoss, got %v", classified)
	}
}

func TestClassifyError_Locked(t *testing.T) {
	pe := &api.ProxmoxError{StatusCode: 500, Message: "VM is locked (backup)"}
	classified := api.ClassifyError(pe)
	if !errors.Is(classified, api.ErrLocked) {
		t.Errorf("expected ErrLocked, got %v", classified)
	}
}

func TestClassifyError_Generic500(t *testing.T) {
	pe := &api.ProxmoxError{StatusCode: 500, Message: "something else"}
	classified := api.ClassifyError(pe)
	// Generic 500 should be classified as retryable ProxmoxError
	if api.IsRetryable(classified) != true {
		t.Error("generic 500 should be retryable")
	}
}

func TestClassifyError_Nil(t *testing.T) {
	if api.ClassifyError(nil) != nil {
		t.Error("ClassifyError(nil) should be nil")
	}
}

func TestRedactMessage_UUID(t *testing.T) {
	msg := "token 12345678-1234-1234-1234-123456789abc expired"
	redacted := api.RedactMessage(msg)
	if strings.Contains(redacted, "12345678-1234") {
		t.Error("UUID should be redacted")
	}
	if !strings.Contains(redacted, "REDACTED") {
		t.Error("should contain REDACTED")
	}
}

func TestRedactMessage_TokenHeader(t *testing.T) {
	msg := "auth failed for PVEAPIToken=root@pam!mytoken=secret-uuid-here"
	redacted := api.RedactMessage(msg)
	if strings.Contains(redacted, "secret-uuid") {
		t.Error("token should be redacted")
	}
}

func TestRedactMessage_Truncate(t *testing.T) {
	long := strings.Repeat("x", 600)
	redacted := api.RedactMessage(long)
	if len(redacted) > 520 {
		t.Errorf("redacted message too long: %d", len(redacted))
	}
}

// ── Timeouts ──────────────────────────────────────────────────────────────────

func TestTimeoutConstants(t *testing.T) {
	if api.TimeoutQuick >= api.TimeoutStandard {
		t.Error("TimeoutQuick should be less than TimeoutStandard")
	}
	if api.TimeoutStandard >= api.TimeoutLong {
		t.Error("TimeoutStandard should be less than TimeoutLong")
	}
	if api.TimeoutLong >= api.TimeoutMigration {
		t.Error("TimeoutLong should be less than TimeoutMigration")
	}
	if api.TimeoutQuick <= 0 {
		t.Error("TimeoutQuick should be positive")
	}
}

// ── Firewall rules ────────────────────────────────────────────────────────────

func TestGetClusterFirewallRules(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/cluster/firewall/rules": jsonOK([]map[string]any{
			{"pos": 0, "action": "ACCEPT", "type": "in", "enable": 1, "comment": "Allow all"},
		}),
	})
	rules, err := c.GetClusterFirewallRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Action != "ACCEPT" {
		t.Errorf("unexpected rules: %+v", rules)
	}
}

func TestGetNodeFirewallRules(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/firewall/rules": jsonOK([]map[string]any{
			{"pos": 0, "action": "ACCEPT", "type": "in"},
		}),
	})
	rules, err := c.GetNodeFirewallRules(context.Background(), "pve1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestGetVMFirewallRules(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/qemu/100/firewall/rules": jsonOK([]map[string]any{
			{"pos": 0, "action": "DROP", "type": "in", "dest": "10.0.0.0/8"},
		}),
	})
	rules, err := c.GetVMFirewallRules(context.Background(), "pve1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Action != "DROP" {
		t.Errorf("unexpected rules: %+v", rules)
	}
}

// ── Pool CRUD ─────────────────────────────────────────────────────────────────

func TestCreatePool(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/pools": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.CreatePool(context.Background(), "testpool", "my pool")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestDeletePool(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/pools/testpool": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.DeletePool(context.Background(), "testpool")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q", gotMethod)
	}
}

// ── Group/Role CRUD ───────────────────────────────────────────────────────────

func TestCreateGroup(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/groups": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.CreateGroup(context.Background(), "devops", "DevOps team")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
}

func TestDeleteGroup(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/groups/devops": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.DeleteGroup(context.Background(), "devops")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRole(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/roles": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.CreateRole(context.Background(), "MyRole", "VM.Allocate")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRole(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/roles/MyRole": jsonOK(nil),
	})
	err := c.DeleteRole(context.Background(), "MyRole")
	if err != nil {
		t.Fatal(err)
	}
}

// ── ACL Update ────────────────────────────────────────────────────────────────

func TestUpdateACL(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/access/acl": captureMethod(t, &gotMethod, new(string)),
	})
	err := c.UpdateACL(context.Background(), "/", "root@pam", "user", "PVEAdmin", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "PUT" {
		t.Errorf("method = %q", gotMethod)
	}
}

// ── CloneCT ───────────────────────────────────────────────────────────────────

func TestCloneCT(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/lxc/200/clone": captureMethod(t, &gotMethod, new(string)),
	})
	upid, err := c.CloneCT(context.Background(), "pve1", 200, 201, "dns-clone")
	if err != nil {
		t.Fatal(err)
	}
	if upid != "OK" {
		t.Errorf("UPID = %q", upid)
	}
}

// ── GetVolumes with content filter ────────────────────────────────────────────

func TestGetVolumes_WithContent(t *testing.T) {
	var gotQuery string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage/local-lvm/content": func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		},
	})
	_, err := c.GetVolumes(context.Background(), "pve1", "local-lvm", "images")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "content=images" {
		t.Errorf("query = %q", gotQuery)
	}
}

// ── Ping error ────────────────────────────────────────────────────────────────

func TestPing_ServerError(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/version": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte(`{"errors":{"detail":"internal error"}}`))
		},
	})
	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Error paths ───────────────────────────────────────────────────────────────

func TestGetNodes_ServerError(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte(`{"errors":{"detail":"server error"}}`))
		},
	})
	_, err := c.GetNodes(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateBackup(t *testing.T) {
	var gotMethod string
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{
			{"storage": "backup", "type": "dir", "active": 1, "content": "backup,iso", "total": 1099511627776, "used": 0},
		}),
		"/api2/json/nodes/pve1/vzdump": captureMethod(t, &gotMethod, new(string)),
	})
	upid, err := c.CreateBackup(context.Background(), "pve1", 100, "")
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

func TestCreateBackup_ExplicitStorage(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/vzdump": jsonUPID(),
	})
	upid, err := c.CreateBackup(context.Background(), "pve1", 100, "my-backup")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(upid, "UPID") {
		t.Errorf("UPID = %q", upid)
	}
}

func TestCreateBackup_NoStorage(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/pve1/storage": jsonOK([]map[string]any{}),
	})
	_, err := c.CreateBackup(context.Background(), "pve1", 100, "")
	if err == nil {
		t.Fatal("expected error when no backup storage found")
	}
}

func TestGetNodeStatus_Error(t *testing.T) {
	_, c := newRoutingServer(t, map[string]http.HandlerFunc{
		"/api2/json/nodes/bad/status": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte("error"))
		},
	})
	_, err := c.GetNodeStatus(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIsRetryable_SentinelErrors(t *testing.T) {
	if api.IsRetryable(api.ErrUnauthorized) {
		t.Error("ErrUnauthorized should not be retryable")
	}
	if api.IsRetryable(api.ErrForbidden) {
		t.Error("ErrForbidden should not be retryable")
	}
	if api.IsRetryable(api.ErrNotFound) {
		t.Error("ErrNotFound should not be retryable")
	}
	if !api.IsRetryable(api.ErrLocked) {
		t.Error("ErrLocked should be retryable")
	}
	if !api.IsRetryable(api.ErrQuorumLoss) {
		t.Error("ErrQuorumLoss should be retryable")
	}
	if api.IsRetryable(nil) {
		t.Error("nil should not be retryable")
	}
}

func TestClassifyError_AuthErrors(t *testing.T) {
	pe401 := &api.ProxmoxError{StatusCode: 401, Message: "no auth"}
	if api.ClassifyError(pe401) != api.ErrUnauthorized {
		t.Error("401 should be ErrUnauthorized")
	}
	pe403 := &api.ProxmoxError{StatusCode: 403, Message: "forbidden"}
	if api.ClassifyError(pe403) != api.ErrForbidden {
		t.Error("403 should be ErrForbidden")
	}
	pe404 := &api.ProxmoxError{StatusCode: 404, Message: "gone"}
	if api.ClassifyError(pe404) != api.ErrNotFound {
		t.Error("404 should be ErrNotFound")
	}
}

func TestClassifyError_NonProxmoxError(t *testing.T) {
	stdErr := errors.New("standard error")
	if api.ClassifyError(stdErr) != stdErr {
		t.Error("non-ProxmoxError should pass through")
	}
}
