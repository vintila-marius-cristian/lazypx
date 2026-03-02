package tui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/lipgloss"

	"lazypx/api"
	"lazypx/state"
)

// SidebarModel manages the 4 stacked accordion list panels (Nodes, VMs, CTs, Storage).
type SidebarModel struct {
	st *state.AppState

	nodesList   *ListPane
	vmsList     *ListPane
	ctsList     *ListPane
	storageList *ListPane

	width  int
	height int

	nodesH, vmsH, ctsH, storageH int
}

func NewSidebarModel(st *state.AppState) SidebarModel {
	return SidebarModel{
		st:          st,
		nodesList:   NewListPane(state.PanelNodes, "Nodes"),
		vmsList:     NewListPane(state.PanelVMs, "Virtual Machines"),
		ctsList:     NewListPane(state.PanelCTs, "Containers"),
		storageList: NewListPane(state.PanelStorage, "Storage"),
	}
}

// SetSize gives the outer dimensions for each accordion.
func (m *SidebarModel) SetSize(layout Layout) {
	m.width = layout.SidebarOuterW
	m.height = layout.SidebarOuterH

	m.nodesH = layout.NodesOuterH
	m.vmsH = layout.VMsOuterH
	m.ctsH = layout.CTsOuterH
	m.storageH = layout.StorageOuterH

	m.nodesList.SetSize(m.width, m.nodesH)
	m.vmsList.SetSize(m.width, m.vmsH)
	m.ctsList.SetSize(m.width, m.ctsH)
	m.storageList.SetSize(m.width, m.storageH)
}

// Sync builds the 4 lists from the cluster snapshot.
// It applies relational filtering: VMs/CTs/Storage only show items for the currently selected Node.
func (m *SidebarModel) Sync(st *state.AppState) {
	m.st = st
	snap := m.st.Snapshot

	// 1. Nodes
	var nItems []ListItem
	for i := range snap.Nodes {
		n := snap.Nodes[i]
		nItems = append(nItems, ListItem{
			ID:      n.Node,
			Label:   n.Node,
			Name:    "",
			Status:  n.Status,
			RawData: &snap.Nodes[i],
		})
	}
	m.nodesList.Items = nItems
	m.nodesList.clampCursor()

	// Find the currently selected Node Name (from Nodes list)
	selectedNode := ""
	if item := m.nodesList.SelectedItem(); item != nil {
		selectedNode = item.ID
		m.st.SelectedNode = selectedNode
	}

	// If no node selected, empty the lower lists.
	if selectedNode == "" {
		m.vmsList.Items = nil
		m.ctsList.Items = nil
		m.storageList.Items = nil
		m.UpdateSelection()
		return
	}

	// 2. VMs (filtered by selectedNode)
	var vItems []ListItem
	vms := snap.VMs[selectedNode]
	// Sort by VMID
	sort.Slice(vms, func(a, b int) bool { return vms[a].VMID < vms[b].VMID })
	for i := range vms {
		vm := vms[i]
		status := ""
		if vm.Status == "running" {
			status = "running"
		}
		vItems = append(vItems, ListItem{
			ID:      fmt.Sprintf("%d", vm.VMID),
			Label:   fmt.Sprintf("%d", vm.VMID),
			Name:    vm.Name,
			Status:  status,
			RawData: &vms[i],
		})
	}
	m.vmsList.Items = vItems
	m.vmsList.clampCursor()

	// 3. CTs
	var cItems []ListItem
	cts := snap.Containers[selectedNode]
	sort.Slice(cts, func(a, b int) bool { return cts[a].VMID < cts[b].VMID })
	for i := range cts {
		ct := cts[i]
		status := ""
		if ct.Status == "running" {
			status = "running"
		}
		cItems = append(cItems, ListItem{
			ID:      fmt.Sprintf("%d", ct.VMID),
			Label:   fmt.Sprintf("%d", ct.VMID),
			Name:    ct.Name,
			Status:  status,
			RawData: &cts[i],
		})
	}
	m.ctsList.Items = cItems
	m.ctsList.clampCursor()

	// 4. Storage
	var sItems []ListItem
	storage := snap.Storage[selectedNode]
	sort.Slice(storage, func(a, b int) bool { return storage[a].Storage < storage[b].Storage })
	for i := range storage {
		s := storage[i]
		sItems = append(sItems, ListItem{
			ID:      s.Storage,
			Label:   s.Storage,
			Name:    "", // no sub-name
			Status:  s.Status,
			RawData: &storage[i],
		})
	}
	m.storageList.Items = sItems
	m.storageList.clampCursor()

	m.UpdateSelection()
}

// ApplyFilter filters items across all lists (not fully implemented yet, just sync)
func (m *SidebarModel) ApplyFilter(query string, st *state.AppState) {
	// For now, simpler: just clear query
	if query == "" {
		m.Sync(st)
	}
}

func (m *SidebarModel) ActiveList() *ListPane {
	switch m.st.FocusedPanel {
	case state.PanelNodes:
		return m.nodesList
	case state.PanelVMs:
		return m.vmsList
	case state.PanelCTs:
		return m.ctsList
	case state.PanelStorage:
		return m.storageList
	default:
		// If detail/tasks focused, treat Nodes as active for cursor moving
		return m.nodesList
	}
}

func (m *SidebarModel) MoveUp() {
	m.ActiveList().MoveUp()
	m.UpdateSelection()
	// If manipulating node list, cascade sync downstream
	if m.st.FocusedPanel == state.PanelNodes {
		m.Sync(m.st)
	}
}

func (m *SidebarModel) MoveDown() {
	m.ActiveList().MoveDown()
	m.UpdateSelection()
	if m.st.FocusedPanel == state.PanelNodes {
		m.Sync(m.st)
	}
}

// UpdateSelection surfaces the focused item to the global State so detail.go can render it.
func (m *SidebarModel) UpdateSelection() {
	m.st.Selected = state.Selection{Kind: state.KindNone}

	switch m.st.FocusedPanel {
	case state.PanelNodes:
		if item := m.nodesList.SelectedItem(); item != nil {
			m.st.Selected = state.Selection{
				Kind:       state.KindNode,
				NodeName:   item.ID,
				NodeStatus: item.RawData.(*api.NodeStatus),
			}
		}
	case state.PanelVMs:
		if item := m.vmsList.SelectedItem(); item != nil {
			vm := item.RawData.(*api.VMStatus)
			m.st.Selected = state.Selection{
				Kind:     state.KindVM,
				NodeName: vm.Node,
				VMID:     vm.VMID,
				VMStatus: vm,
			}
		}
	case state.PanelCTs:
		if item := m.ctsList.SelectedItem(); item != nil {
			ct := item.RawData.(*api.CTStatus)
			m.st.Selected = state.Selection{
				Kind:     state.KindContainer,
				NodeName: ct.Node,
				VMID:     ct.VMID,
				CTStatus: ct,
			}
		}
	case state.PanelStorage:
		if item := m.storageList.SelectedItem(); item != nil {
			s := item.RawData.(*api.StorageStatus)
			m.st.Selected = state.Selection{
				Kind:        state.KindStorage,
				NodeName:    s.Node,
				StorageName: s.Storage,
			}
		}
	}
}

func (m SidebarModel) View(focusedPanel state.PanelType) string {
	nView := m.nodesList.View(focusedPanel == state.PanelNodes)
	vView := m.vmsList.View(focusedPanel == state.PanelVMs)
	cView := m.ctsList.View(focusedPanel == state.PanelCTs)
	sView := m.storageList.View(focusedPanel == state.PanelStorage)

	return lipgloss.JoinVertical(lipgloss.Left, nView, vView, cView, sView)
}
