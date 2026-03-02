package api

// APIResponse is the envelope for all Proxmox REST responses.
type APIResponse[T any] struct {
	Data T `json:"data"`
}

// NodeStatus represents a node in /nodes response.
type NodeStatus struct {
	Node           string  `json:"node"`
	Status         string  `json:"status"` // online | offline
	Type           string  `json:"type"`
	CPUUsage       float64 `json:"cpu"`
	MaxCPU         int     `json:"maxcpu"`
	MemUsed        int64   `json:"mem"`
	MemTotal       int64   `json:"maxmem"`
	DiskUsed       int64   `json:"disk"`
	DiskTotal      int64   `json:"maxdisk"`
	Uptime         int64   `json:"uptime"`
	SSLFingerprint string  `json:"ssl_fingerprint,omitempty"`
	// Extended is populated by GetNodeStatus (not GetNodes)
	Extended *NodeStatusExtended `json:"-"`
}

// NodeStatusExtended holds richer info from /nodes/{node}/status (nested fields).
type NodeStatusExtended struct {
	PVEVersion  string
	KernelVer   string
	CPUModel    string
	CPUCores    int
	CPUSockets  int
	CPUMHz      string
	SwapUsed    int64
	SwapTotal   int64
	RootFSAvail int64
}

// VMStatus represents a VM from /nodes/{node}/qemu response.
type VMStatus struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Status    string  `json:"status"` // running | stopped | suspended
	Node      string  `json:"-"`      // filled in by client
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"cpus"`
	MemUsed   int64   `json:"mem"`
	MemTotal  int64   `json:"maxmem"`
	DiskWrite int64   `json:"diskwrite"`
	DiskRead  int64   `json:"diskread"`
	DiskUsed  int64   `json:"disk"`    // root disk used bytes (running VMs only)
	MaxDisk   int64   `json:"maxdisk"` // root disk size bytes
	NetOut    int64   `json:"netout"`
	NetIn     int64   `json:"netin"`
	Uptime    int64   `json:"uptime"`
	Tags      string  `json:"tags"`
	Template  int     `json:"template"`
	Lock      string  `json:"lock,omitempty"`
	QMPStatus string  `json:"qmpstatus,omitempty"`
	HA        *HAInfo `json:"ha,omitempty"`
}

// HAInfo carries HA state embedded in VM/CT status responses.
type HAInfo struct {
	Managed int    `json:"managed"`
	State   string `json:"state,omitempty"`
}

// CTStatus represents a container from /nodes/{node}/lxc response.
type CTStatus struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Node     string  `json:"-"`
	CPU      float64 `json:"cpu"`
	MaxCPU   int     `json:"cpus"`
	MemUsed  int64   `json:"mem"`
	MemTotal int64   `json:"maxmem"`
	DiskUsed int64   `json:"disk"`
	DiskMax  int64   `json:"maxdisk"`
	NetOut   int64   `json:"netout"`
	NetIn    int64   `json:"netin"`
	Uptime   int64   `json:"uptime"`
	Tags     string  `json:"tags"`
	Type     string  `json:"type"`
}

// StorageStatus represents storage from /nodes/{node}/storage response.
type StorageStatus struct {
	Storage      string  `json:"storage"`
	Node         string  `json:"-"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Used         int64   `json:"used"`
	Total        int64   `json:"total"`
	Avail        int64   `json:"avail"`
	Active       int     `json:"active"`
	Enabled      int     `json:"enabled"`
	Shared       int     `json:"shared"`
	Content      string  `json:"content"`       // e.g. "images,rootdir,backup"
	UsedFraction float64 `json:"used_fraction"` // 0.0–1.0
}

// Task represents a Proxmox task from /nodes/{node}/tasks.
type Task struct {
	UPID      string  `json:"upid"`
	Node      string  `json:"node"`
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	User      string  `json:"user"`
	Status    string  `json:"status"`
	StartTime float64 `json:"starttime"`
	EndTime   float64 `json:"endtime,omitempty"`
}

// TaskLog is a single log line from a task.
type TaskLog struct {
	N int    `json:"n"`
	T string `json:"t"`
}

// TaskStatus is the current state of a running task.
type TaskStatusResponse struct {
	Status     string `json:"status"` // running | stopped
	ExitStatus string `json:"exitstatus,omitempty"`
}
