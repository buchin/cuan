package agent

import "github.com/buchin/agentrunner"

// Keep Cuan's call sites stable while sharing the tested process primitives.
// Account selection, task deadlines and completion fencing remain in Cuan.
func KillProcessTree(pid int) error { return agentrunner.KillProcessTree(pid) }
func GetChildPIDs(pid int) []int    { return agentrunner.GetChildPIDs(pid) }
func IsProcessAlive(pid int) bool   { return agentrunner.IsProcessAlive(pid) }
