package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// JSON-RPC 客户端
type JSONRPCClient struct {
	url    string
	client *http.Client
	nextID int
}

// 创建新的 JSON-RPC 客户端
func NewJSONRPCClient(url string) *JSONRPCClient {
	return &JSONRPCClient{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		nextID: 1,
	}
}

// 发送 JSON-RPC 请求
func (c *JSONRPCClient) call(method string, params interface{}) (*JSONRPCResponse, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID,
	}
	c.nextID++

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := c.client.Post(c.url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	return &jsonResp, nil
}

// 获取状态
func (c *JSONRPCClient) Status() (*StatusReply, error) {
	resp, err := c.call("status", nil)
	if err != nil {
		return nil, err
	}

	var result StatusReply
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}

	return &result, nil
}

// 获取对等节点列表
func (c *JSONRPCClient) Peers() (*PeersReply, error) {
	resp, err := c.call("peers", nil)
	if err != nil {
		return nil, err
	}

	var result PeersReply
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}

	return &result, nil
}

// 路由操作
func (c *JSONRPCClient) Route(action RouteAction, args []string) (*RouteReply, error) {
	params := map[string]interface{}{
		"action": string(action),
		"args":   args,
	}

	resp, err := c.call("route", params)
	if err != nil {
		return nil, err
	}

	var result RouteReply
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}

	return &result, nil
}

// 添加对等节点
func (c *JSONRPCClient) AddPeer(name, id string) (*AddPeerReply, error) {
	params := map[string]interface{}{
		"name": name,
		"id":   id,
	}

	resp, err := c.call("addPeer", params)
	if err != nil {
		return nil, err
	}

	var result AddPeerReply
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}

	return &result, nil
}

// 获取当前节点的 IP 地址
func (c *JSONRPCClient) NodeIp() (*NodeIpReply, error) {
	resp, err := c.call("nodeIp", nil)
	if err != nil {
		return nil, err
	}

	var result NodeIpReply
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %v", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %v", err)
	}

	return &result, nil
}

// 便捷方法：显示路由
func (c *JSONRPCClient) ShowRoutes() (*RouteReply, error) {
	return c.Route(Show, nil)
}

// 便捷方法：添加路由
func (c *JSONRPCClient) AddRoute(destination string) (*RouteReply, error) {
	return c.Route(Add, []string{destination})
}

// 便捷方法：删除路由
func (c *JSONRPCClient) DelRoute(destination string) (*RouteReply, error) {
	return c.Route(Del, []string{destination})
}