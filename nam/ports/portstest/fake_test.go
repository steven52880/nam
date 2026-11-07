package portstest

import "testing"

func TestFakeContract(t *testing.T) {
	RunContract(t, ContractEnv{Ports: NewFake().Ports()})
}
