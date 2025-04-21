package tunnelserver

import (
	"fmt"
	"net/netip"
	"testing"

	v1 "k8s.io/api/core/v1"
)

type MockServer struct {
	AddedPeers []struct {
		URL   string
		ID    string
		Token string
	}
	PeerID string
}

type peerManagerMock struct {
	server    *MockServer
	token     string
	urlFormat string
}

func (m *MockServer) AddPeer(url, id, token string) {
	m.AddedPeers = append(m.AddedPeers, struct {
		URL   string
		ID    string
		Token string
	}{url, id, token})
}

func (pm *peerManagerMock) addRemovePeers(endpoints *v1.Endpoints) {
	toCreate := []string{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			toCreate = append(toCreate, addr.IP)
		}
	}

	for _, ip := range toCreate {
		displayIP := ip
		ipAddr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		if ipAddr.Is6() {
			displayIP = fmt.Sprintf("[%s]", ip)
		}
		pm.server.AddPeer(fmt.Sprintf(pm.urlFormat, displayIP), ip, "test-token")
	}
}

func TestURLFormatting(t *testing.T) {
	testCases := []struct {
		name        string
		peerID      string
		endpoints   v1.Endpoints
		expectedURL []string
	}{
		{
			name:   "valid IPv4 address",
			peerID: "ipv4",
			endpoints: v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "192.0.0.1"},
							{IP: "192.0.0.2"},
						},
					},
				},
			},
			expectedURL: []string{"ws://192.0.0.1/v3/connect", "ws://192.0.0.2/v3/connect"},
		},
		{
			name:   "valid IPv6 address",
			peerID: "ipv6",
			endpoints: v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "2001:db8::1"},
						},
					},
				},
			},
			expectedURL: []string{"ws://[2001:db8::1]/v3/connect"},
		},
		{
			name:   "mixed IPv4 and IPv6 addresses",
			peerID: "mixed",
			endpoints: v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "192.0.0.1"},
							{IP: "2001:db8::1"},
							{IP: "10.0.0.1"},
						},
					},
				},
			},
			expectedURL: []string{
				"ws://192.0.0.1/v3/connect",
				"ws://[2001:db8::1]/v3/connect",
				"ws://10.0.0.1/v3/connect",
			},
		},
		{
			name:   "invalid IP",
			peerID: "invalid ip",
			endpoints: v1.Endpoints{
				Subsets: []v1.EndpointSubset{
					{
						Addresses: []v1.EndpointAddress{
							{IP: "192.0."},
						},
					},
				},
			},
			expectedURL: []string{},
		},
		{
			name:        "empty endpoints",
			peerID:      "empty",
			endpoints:   v1.Endpoints{},
			expectedURL: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := MockServer{
				PeerID: tc.peerID,
			}

			pm := &peerManagerMock{
				server:    &mockServer,
				urlFormat: "ws://%s/v3/connect",
			}

			pm.addRemovePeers(&tc.endpoints)
			if len(mockServer.AddedPeers) != len(tc.expectedURL) {
				t.Fatalf("expected %v peers to be added, got %v", len(tc.expectedURL), len(mockServer.AddedPeers))
			}
			for i, val := range mockServer.AddedPeers {
				if tc.expectedURL[i] != val.URL {
					t.Fatalf("expected url to be %v, got %v", tc.expectedURL[i], val.URL)
				}
			}
		})
	}
}
