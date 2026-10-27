package deployer

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

const (
	// DefaultMinPort defines the lower bound of the sandbox port pool.
	DefaultMinPort = 9000

	// DefaultMaxPort defines the upper bound of the sandbox port pool.
	DefaultMaxPort = 9999
)

var (
	// ErrPortPoolExhausted indicates that all ports in the pool are currently leased or bound.
	ErrPortPoolExhausted = errors.New("port pool exhausted: no available ports in range")

	// ErrPortNotAllocated indicates that the specified deployment does not have an active lease.
	ErrPortNotAllocated = errors.New("no active port lease for deployment")
)

// PortPool manages atomic, thread-safe dynamic port leases within a specified port range.
// It ensures zero collision by tracking active leases in memory and checking real TCP socket availability.
type PortPool struct {
	mu          sync.Mutex
	minPort     int
	maxPort     int
	leases      map[int]string // port -> deploymentID
	deployments map[string]int // deploymentID -> port
	checkSocket bool           // verify real TCP socket availability on 127.0.0.1
}

// NewPortPool initializes a PortPool with the default range 9000-9999.
func NewPortPool() *PortPool {
	return NewPortPoolWithRange(DefaultMinPort, DefaultMaxPort)
}

// NewPortPoolWithRange initializes a PortPool with custom port boundaries.
func NewPortPoolWithRange(minPort, maxPort int) *PortPool {
	if minPort <= 0 || maxPort < minPort {
		minPort = DefaultMinPort
		maxPort = DefaultMaxPort
	}
	return &PortPool{
		minPort:     minPort,
		maxPort:     maxPort,
		leases:      make(map[int]string),
		deployments: make(map[string]int),
		checkSocket: true,
	}
}

// SetCheckSocket enables or disables real TCP socket availability verification before leasing.
func (p *PortPool) SetCheckSocket(check bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkSocket = check
}

// Range returns the configured minimum and maximum port bounds.
func (p *PortPool) Range() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.minPort, p.maxPort
}

// Allocate atomically allocates a collision-free port for the given deployment ID.
// If the deployment already holds an active lease, the existing port is returned.
func (p *PortPool) Allocate(deploymentID string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if port, exists := p.deployments[deploymentID]; exists {
		return port, nil
	}

	totalPorts := p.maxPort - p.minPort + 1
	for i := 0; i < totalPorts; i++ {
		port := p.minPort + i
		if _, taken := p.leases[port]; taken {
			continue
		}

		if p.checkSocket {
			if !isSocketAvailable(port) {
				continue
			}
		}

		p.leases[port] = deploymentID
		p.deployments[deploymentID] = port
		return port, nil
	}

	return 0, ErrPortPoolExhausted
}

// Release releases the port leased to the given deployment ID.
func (p *PortPool) Release(deploymentID string) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	port, exists := p.deployments[deploymentID]
	if !exists {
		return 0, false
	}

	delete(p.deployments, deploymentID)
	delete(p.leases, port)
	return port, true
}

// ReleasePort releases a specific port from active leases.
func (p *PortPool) ReleasePort(port int) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	depID, exists := p.leases[port]
	if !exists {
		return "", false
	}

	delete(p.leases, port)
	delete(p.deployments, depID)
	return depID, true
}

// GetPort returns the leased port for a deployment ID if allocated.
func (p *PortPool) GetPort(deploymentID string) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	port, exists := p.deployments[deploymentID]
	return port, exists
}

// IsLeased returns true if the specified port is leased in the pool.
func (p *PortPool) IsLeased(port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, leased := p.leases[port]
	return leased
}

// ActiveLeases returns a copy of all currently leased ports and their owner deployment IDs.
func (p *PortPool) ActiveLeases() map[int]string {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make(map[int]string, len(p.leases))
	for port, depID := range p.leases {
		out[port] = depID
	}
	return out
}

// isSocketAvailable probes 127.0.0.1:<port> to verify TCP bind availability.
func isSocketAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
