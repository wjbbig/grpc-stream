package internal

import "testing"

func TestReadConfig(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := NewServerConfig(config)
	t.Log(serverConfig)
}
