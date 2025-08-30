package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/soitun/mynetwork/config"
	"github.com/soitun/mynetwork/tun"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// JSON-RPC 2.0 标准结构
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// JSON-RPC 错误码
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// JSON-RPC 服务器
type JSONRPCServer struct {
	rpcService *HyprspaceRPC
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 创建新的 JSON-RPC 服务器
func NewJSONRPCServer(host host.Host, config *config.Config, tunDev *tun.TUN) *JSONRPCServer {
	return &JSONRPCServer{
		rpcService: &HyprspaceRPC{
			host:   host,
			config: config,
			tunDev: tunDev,
		},
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// HTTP 处理器
func (s *JSONRPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, ParseError, "Parse error", nil, nil)
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, InvalidRequest, "Invalid Request", nil, req.ID)
		return
	}

	result, err := s.handleMethod(req.Method, req.Params)
	if err != nil {
		s.writeError(w, err.Code, err.Message, err.Data, req.ID)
		return
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}

	json.NewEncoder(w).Encode(response)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理方法调用
func (s *JSONRPCServer) handleMethod(method string, params interface{}) (interface{}, *JSONRPCError) {
	switch method {
	case "status":
		return s.handleStatus(params)
	case "peers":
		return s.handlePeers(params)
	case "route":
		return s.handleRoute(params)
	case "addPeer":
		return s.handleAddPeer(params)
	case "nodeIp":
		return s.handleNodeIp(params)
	default:
		return nil, &JSONRPCError{
			Code:    MethodNotFound,
			Message: "Method not found",
		}
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理 status 方法
func (s *JSONRPCServer) handleStatus(params interface{}) (interface{}, *JSONRPCError) {
	var args Args
	var reply StatusReply

	err := s.rpcService.Status(&args, &reply)
	if err != nil {
		return nil, &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    err.Error(),
		}
	}

	return reply, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理 peers 方法
func (s *JSONRPCServer) handlePeers(params interface{}) (interface{}, *JSONRPCError) {
	var args Args
	var reply PeersReply

	err := s.rpcService.Peers(&args, &reply)
	if err != nil {
		return nil, &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    err.Error(),
		}
	}

	return reply, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理 route 方法
func (s *JSONRPCServer) handleRoute(params interface{}) (interface{}, *JSONRPCError) {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return nil, &JSONRPCError{
			Code:    InvalidParams,
			Message: "Invalid params",
		}
	}

	var args RouteArgs

	// 解析 action
	if actionStr, ok := paramsMap["action"].(string); ok {
		args.Action = RouteAction(actionStr)
	} else {
		args.Action = Show // 默认为 show
	}

	// 解析 args
	if argsInterface, ok := paramsMap["args"]; ok {
		if argsList, ok := argsInterface.([]interface{}); ok {
			for _, arg := range argsList {
				if argStr, ok := arg.(string); ok {
					args.Args = append(args.Args, argStr)
				}
			}
		}
	}

	var reply RouteReply
	err := s.rpcService.Route(&args, &reply)
	if err != nil {
		return nil, &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    err.Error(),
		}
	}

	return reply, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理 addPeer 方法
func (s *JSONRPCServer) handleAddPeer(params interface{}) (interface{}, *JSONRPCError) {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return nil, &JSONRPCError{
			Code:    InvalidParams,
			Message: "Invalid params",
		}
	}

	var args AddPeerArgs

	// 解析 name
	if name, ok := paramsMap["name"].(string); ok {
		args.Name = name
	} else {
		return nil, &JSONRPCError{
			Code:    InvalidParams,
			Message: "Missing or invalid 'name' parameter",
		}
	}

	// 解析 id
	if id, ok := paramsMap["id"].(string); ok {
		args.ID = id
	} else {
		return nil, &JSONRPCError{
			Code:    InvalidParams,
			Message: "Missing or invalid 'id' parameter",
		}
	}

	var reply AddPeerReply
	err := s.rpcService.AddPeer(&args, &reply)
	if err != nil {
		return nil, &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    err.Error(),
		}
	}

	return reply, nil
}

// 写入错误响应}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 处理 nodeIp 方法
func (s *JSONRPCServer) handleNodeIp(params interface{}) (interface{}, *JSONRPCError) {
	var args Args
	var reply NodeIpReply
	err := s.rpcService.NodeIp(&args, &reply)
	if err != nil {
		return nil, &JSONRPCError{
			Code:    InternalError,
			Message: "Internal error",
			Data:    err.Error(),
		}
	}

	return reply, nil
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func (s *JSONRPCServer) writeError(w http.ResponseWriter, code int, message string, data interface{}, id interface{}) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
	json.NewEncoder(w).Encode(response)
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
// 启动 JSON-RPC 服务器
func StartJSONRPCServer(ctx context.Context, host host.Host, config *config.Config, tunDev *tun.TUN) {
	hjr := NewJSONRPCServer(host, config, tunDev)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.Handle("/", hjr)

	server := &http.Server{
		Handler: mux,
	}

	// Determine listen address based on platform
	var listener net.Listener
	var err error

	// For cross-platform compatibility, use TCP on a random port
	listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("[!] Failed to create JSON-RPC server listener: ", err)
	}

	// Write port to file so clients can find it
	tcpAddr := listener.Addr().(*net.TCPAddr)
	portFile := filepath.Join(os.TempDir(), fmt.Sprintf("mynetwork-jsonrpc.%s.port", config.Interface))
	err = os.WriteFile(portFile, []byte(strconv.Itoa(tcpAddr.Port)), 0644)
	if err != nil {
		log.Printf("[!] Warning: Could not write JSON-RPC port file: %s", err)
	}

	fmt.Printf("[-] JSON-RPC 2.0 server ready on http://%s\n", listener.Addr().String())
	fmt.Printf("[-] JSON-RPC port file: %s\n", portFile)

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[!] JSON-RPC server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	fmt.Printf("[-] Shutting down JSON-RPC server...\n")
	server.Shutdown(context.Background())
}
