# Mynetwork Delphi 客户端示例

本目录包含 Delphi 示例，用于直接连接到 Mynetwork 的 JSON-RPC 2.0 服务器。

## rpc.dpr - JSON-RPC 2.0 直连客户端

这是一个完整的 Delphi 控制台程序，实现了与 Mynetwork JSON-RPC 服务器的直接连接。

### 主要特性

- **自动端口发现**：通过读取临时文件或扫描常见端口自动发现 JSON-RPC 服务器
- **完整的 API 支持**：支持 Status、Peers、AddPeer 等所有主要 RPC 方法
- **JSON-RPC 2.0 标准**：完全符合 JSON-RPC 2.0 协议规范
- **错误处理**：完善的连接和调用错误处理机制
- **Windows 兼容**：专为 Windows 平台优化

### 支持的 RPC 方法

1. **Status** - 获取节点状态信息
2. **Peers** - 获取当前对等节点列表
3. **AddPeer** - 添加新的对等节点

### 使用方法

1. 确保 Mynetwork 服务正在运行
2. 编译并运行 `rpc.dpr`
3. 程序将自动连接到 JSON-RPC 服务器并演示所有 API 调用

### 端口发现机制

程序使用以下策略发现 JSON-RPC 服务器：

1. 读取端口文件：`%TEMP%\mynetwork-jsonrpc.hs0.port`
2. 如果端口文件不存在，扫描端口范围：8080-8179
3. 对每个候选端口进行连接测试
4. 使用第一个响应正常的端口

### 技术实现

- **HTTP 客户端**：使用 `System.Net.HttpClient` 进行 HTTP 通信
- **JSON 处理**：使用 `System.JSON` 进行 JSON-RPC 请求/响应处理
- **文件操作**：使用 `System.IOUtils` 进行端口文件读取
- **错误处理**：完善的异常捕获和用户友好的错误信息

## 使用方法

### 1. 启动 Mynetwork 服务

首先确保 Mynetwork 服务正在运行：

```bash
# 启动 Mynetwork
mynetwork up -c /path/to/your/config.json

# 或者使用接口名称（推荐，确保接口名称一致）
mynetwork up -i your-interface-name
```

**重要：** 在 Windows Mynetwork 使用 TCP 连接而不是 Unix socket。服务器会：
- 随机选择一个可用端口（通常在 9000-9099 范围内）
- 将端口号写入文件：`%TEMP%\mynetwork-rpc.{接口名}.port`
- RPC 客户端通过读取此文件来发现正确的端口

### 2. 确保配置文件存在

确保你的 Mynetwork 配置文件存在并且格式正确。

```bash
# 如果没有配置文件，先初始化
mynetwork init
```

### 3. 编译和运行示例

推荐使用 `delphi_cli_addpeer.pas`（完全兼容 Windows）：

```pascal
// 编译并运行
// 在 Delphi IDE 中打开文件并编译，或使用命令行编译器
```

对于 `delphi_addpeer_example.pas`（TCP RPC 版本）：

```bash
# 运行示例，指定接口名称（可选，默认为 'myeth'）
AddPeerExample.exe [interface-name]

# 例如：
AddPeerExample.exe mynetwork
```

**修改配置：**
```pascal
// 在代码中修改这些变量
MynetworkExe := 'C:\path\to\mynetwork.exe'; // Mynetwork 可执行文件路径
ConfigFile := 'C:\path\to\config.json';    // 配置文件路径（可选）
PeerName := 'my-peer';                     // Peer 名称
PeerID := '12D3KooW...';                   // 实际的 Peer ID
```

**编译运行：**
```bash
# 使用 Delphi IDE 编译，或命令行
dcc32 delphi_cli_addpeer.pas
delphi_cli_addpeer.exe
```

## 命令行调用格式

简化后的 add-peer 命令格式：
```bash
mynetwork add-peer --name "peer-name" --id "peer-id"
# 或使用短参数
mynetwork add-peer -n "peer-name" -i "peer-id"
```

## 示例输出

成功添加 peer 时的输出：
```
✓ Peer added successfully
  Peer ID: 12D3KooWMo5jxdx1QycKXx8r7YDGmWq6CZUWGHLmsutnfx1eGqoc
  Peer added temporarily (not saved to config)
```

## 注意事项

1. **服务状态**
   - 确保 Mynetwork 守护进程正在运行
   - 可以通过 `mynetwork status` 检查状态

2. **接口名称一致性**：确保客户端和服务端使用相同的接口名称
   - 服务端：`mynetwork up -i mynetwork`
   - 客户端：`AddPeerExample.exe mynetwork`

3. **Windows TCP 连接**：
   - Windows 下使用 TCP 而不是 Unix socket
   - 端口文件位置：`%TEMP%\mynetwork-rpc.{接口名}.port`
   - 如果端口文件不存在，客户端会扫描 9000-9099 端口范围
   - 连接可能需要几秒钟，客户端会自动重试

4. **Peer ID 格式**
   - 必须是有效的 libp2p Peer ID
   - 通常以 `12D3KooW` 开头

5. **临时性**
   - 添加的 peer 只存在于运行时
   - 重启 Mynetwork 后需要重新添加

6. **错误处理**
   - 检查命令返回值和输出
   - 处理网络连接异常
   - 验证输入参数格式

## 故障排除

### 常见错误

1. **"无法连接到 RPC 服务器"**
   - 检查 Mynetwork 是否正在运行：`mynetwork status`
   - 确认接口名称一致（服务端和客户端）
   - 检查端口文件是否存在：`%TEMP%\mynetwork-rpc.{接口名}.port`
   - 检查防火墙设置（TCP 端口访问）

2. **"端口文件不存在或连接失败"**
   - 确保 Mynetwork 服务已完全启动
   - 检查临时目录权限：`%TEMP%`
   - 手动检查端口文件内容
   - 尝试手动连接到显示的端口

3. **"Peer ID 无效"**
   - 确保 Peer ID 格式正确（libp2p 格式）
   - 检查 Peer ID 长度和字符

4. **"权限被拒绝"**
   - 确保有足够的权限访问配置文件
   - 检查临时目录访问权限

### 调试建议

1. **启用详细日志**
   ```bash
   mynetwork up --verbose -i your-interface-name
   ```

2. **检查服务状态**
   ```bash
   mynetwork status -i your-interface-name
   ```

3. **验证配置**
   ```bash
   mynetwork config show
   ```

4. **手动检查端口文件**
   ```cmd
   # Windows 命令提示符
   type %TEMP%\mynetwork-rpc.your-interface-name.port
   
   # PowerShell
   Get-Content $env:TEMP\mynetwork-rpc.your-interface-name.port
   ```

5. **测试 TCP 连接**
   ```cmd
   # 使用 telnet 测试连接（替换端口号）
   telnet 127.0.0.1 9001
   ```

### 示例输出

```
Mynetwork JSON-RPC 客户端示例
================================

成功连接到 JSON-RPC 服务器: http://127.0.0.1:8080

1. 获取节点状态...
  成功: True
  消息: Node status retrieved successfully
  节点信息: {"id":"...","addresses":[...]}

2. 获取对等节点列表...
  成功: True
  消息: Peers retrieved successfully
  对等节点数量: 2
    [0] {"name":"peer1","id":"..."}
    [1] {"name":"peer2","id":"..."}

3. 添加新的对等节点...
  节点名称: test-peer-delphi
  节点 ID: delphi-example-peer-20241220143022
  成功: True
  消息: Peer added successfully
  返回的 Peer ID: delphi-example-peer-20241220143022

所有操作完成！
按任意键退出...
```

### 编译要求

- Delphi 10.3 Rio 或更高版本
- Windows 平台
- 需要以下单元：
  - System.SysUtils
  - System.JSON
  - System.Net.HttpClient
  - System.IOUtils

## 扩展功能

可以基于这些示例扩展更多功能：

- 批量添加 peer
- 从文件读取 peer 列表
- 集成到现有的 Delphi 应用程序
- 添加 GUI 界面
- 实现 peer 管理功能

## 相关文档

- [Mynetwork 官方文档](https://github.com/soitun/mynetwork)
- [libp2p Peer ID 规范](https://docs.libp2p.io/concepts/peer-id/)
- [JSON-RPC 2.0 规范](https://www.jsonrpc.org/specification)