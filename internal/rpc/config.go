package rpc

import (
	"encoding/json"
	"os"
	"sync"
)

var (
	rpcIdsPathOverride string
	loaded             map[string]string
	loadOnce           sync.Once
	loadMu             sync.Mutex
)

func SetRpcIdsPath(path string) {
	loadMu.Lock()
	defer loadMu.Unlock()
	rpcIdsPathOverride = path
	loaded = nil
	loadOnce = sync.Once{}
}

func LoadRpcIDOverrides() map[string]string {
	loadMu.Lock()
	defer loadMu.Unlock()
	loadOnce.Do(func() {
		loaded = loadFromDisk()
	})
	return loaded
}

func ReloadRpcIDOverrides() map[string]string {
	loadMu.Lock()
	defer loadMu.Unlock()
	loadOnce = sync.Once{}
	loaded = nil
	loadOnce.Do(func() {
		loaded = loadFromDisk()
	})
	return loaded
}

func ResolveRpcID(staticID string, overrides map[string]string) string {
	if overrides != nil {
		if v, ok := overrides[staticID]; ok {
			return v
		}
	}
	return staticID
}

func GetRpcIdsPath() string {
	if rpcIdsPathOverride != "" {
		return rpcIdsPathOverride
	}
	return RpcIDsPath()
}

func loadFromDisk() map[string]string {
	path := GetRpcIdsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]string{}
	}
	return m
}
