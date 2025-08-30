program rpc;

{$APPTYPE CONSOLE}

{$R *.res}

uses
  System.SysUtils,
  System.Classes,
  System.JSON,
  System.Net.HttpClient,
  System.Net.HttpClientComponent,
  System.IOUtils;

type
  // AddPeer RPC 参数结构
  TAddPeerArgs = record
    Name: string;
    ID: string;
  end;

  // AddPeer RPC 响应结构
  TAddPeerReply = record
    Success: Boolean;
    Message: string;
    PeerID: string;
  end;

  // Status RPC 响应结构
  TStatusReply = record
    Success: Boolean;
    Message: string;
    PeerID: string;
    SwarmPeersCurrent: Integer;
    NetPeersCurrent: Integer;
    Node: TJSONArray; // listenAddrs
  end;

  // Peers RPC 响应结构
  TPeersReply = record
    Success: Boolean;
    Message: string;
    Peers: TJSONArray;
  end;

  // GetNodeIP RPC 响应结构
  TGetNodeIPReply = record
    Success: Boolean;
    Message: string;
    NodeIP: string;
    AllIPs: TJSONArray;
    InterfaceName: string;
  end;

  // Hyprspace JSON-RPC 客户端 (直连版本)
  THyprspaceRPCClient = class
  private
    FBaseURL: string;
    FInterfaceName: string;
    function FindJSONRPCServer: string;
    function TestConnection(const BaseURL: string): Boolean;
  public
    constructor Create(const InterfaceName: string = 'hs0');
    destructor Destroy; override;
    function AddPeer(const Args: TAddPeerArgs): TAddPeerReply;
    function Status: TStatusReply;
    function Peers: TPeersReply;
    function GetNodeIP: TGetNodeIPReply;
  end;

constructor THyprspaceRPCClient.Create(const InterfaceName: string);
begin
  inherited Create;
  FInterfaceName := InterfaceName;
  FBaseURL := FindJSONRPCServer;
end;

destructor THyprspaceRPCClient.Destroy;
begin
  inherited Destroy;
end;

function THyprspaceRPCClient.FindJSONRPCServer: string;
var
  PortFile: string;
  PortStr: string;
  Port: Integer;
  BaseURL: string;
  Attempts: Integer;
begin
  Result := '';
  
  // 构建端口文件路径
  PortFile := TPath.Combine(TPath.GetTempPath, Format('mynetwork-jsonrpc.%s.port', [FInterfaceName]));
  
  // 尝试多次，因为服务器可能还在启动
  for Attempts := 1 to 10 do
  begin
    // 尝试从端口文件读取端口
    if TFile.Exists(PortFile) then
    begin
      try
        PortStr := TFile.ReadAllText(PortFile).Trim;
        if TryStrToInt(PortStr, Port) and (Port > 0) then
        begin
          BaseURL := Format('http://127.0.0.1:%d', [Port]);
          if TestConnection(BaseURL) then
          begin
            Result := BaseURL;
            Exit;
          end;
        end;
      except
        // 忽略文件读取错误，继续尝试
      end;
    end;
    
    // 如果端口文件不存在或连接失败，尝试常见端口
    for Port := 8080 to 8179 do
    begin
      BaseURL := Format('http://127.0.0.1:%d', [Port]);
      if TestConnection(BaseURL) then
      begin
        Result := BaseURL;
        Exit;
      end;
    end;
    
    // 等待 500ms 后重试
    Sleep(500);
  end;
  
  if Result = '' then
    raise Exception.Create('无法连接到 JSON-RPC 服务器：找不到服务器');
end;

function THyprspaceRPCClient.TestConnection(const BaseURL: string): Boolean;
var
  HTTPClient: THTTPClient;
  RequestObj: TJSONObject;
  RequestStr: string;
  StringStream: TStringStream;
  Response: IHTTPResponse;
begin
  Result := False;
  HTTPClient := THTTPClient.Create;
  StringStream := TStringStream.Create('', TEncoding.UTF8);
  try
    try
      // 构建简单的 JSON-RPC 测试请求
      RequestObj := TJSONObject.Create;
      try
        RequestObj.AddPair('jsonrpc', '2.0');
        RequestObj.AddPair('method', 'status');
        RequestObj.AddPair('params', TJSONNull.Create);
        RequestObj.AddPair('id', TJSONNumber.Create(1));
        
        RequestStr := RequestObj.ToString;
        StringStream.WriteString(RequestStr);
        StringStream.Position := 0;
        
        HTTPClient.ContentType := 'application/json';
        HTTPClient.ConnectionTimeout := 1000; // 1秒超时
        HTTPClient.ResponseTimeout := 1000;
        
        Response := HTTPClient.Post(BaseURL, StringStream);
        Result := (Response.StatusCode = 200);
      finally
        RequestObj.Free;
      end;
    except
      // 连接失败，返回 False
      Result := False;
    end;
  finally
    StringStream.Free;
    HTTPClient.Free;
  end;
end;



function THyprspaceRPCClient.AddPeer(const Args: TAddPeerArgs): TAddPeerReply;
var
  HTTPClient: THTTPClient;
  RequestObj: TJSONObject;
  ParamsObj: TJSONObject;
  RequestStr: string;
  StringStream: TStringStream;
  Response: IHTTPResponse;
  ResponseObj: TJSONObject;
  ResultObj: TJSONObject;
begin
  // 初始化返回值
  Result.Success := False;
  Result.Message := '';
  Result.PeerID := '';

  HTTPClient := THTTPClient.Create;
  StringStream := TStringStream.Create('', TEncoding.UTF8);
  try
    // 构建 JSON-RPC 2.0 请求
    RequestObj := TJSONObject.Create;
    try
      RequestObj.AddPair('jsonrpc', '2.0');
      RequestObj.AddPair('method', 'addPeer');
      
      // 构建参数对象
      ParamsObj := TJSONObject.Create;
      ParamsObj.AddPair('name', Args.Name);
      ParamsObj.AddPair('id', Args.ID);
      RequestObj.AddPair('params', ParamsObj);
      
      RequestObj.AddPair('id', TJSONNumber.Create(1));
      
      RequestStr := RequestObj.ToString;
      WriteLn('[DEBUG] AddPeer 请求: ', RequestStr);
      StringStream.WriteString(RequestStr);
      StringStream.Position := 0;
      
      HTTPClient.ContentType := 'application/json';
      Response := HTTPClient.Post(FBaseURL, StringStream);
      
      WriteLn('[DEBUG] AddPeer 响应状态码: ', Response.StatusCode);
      WriteLn('[DEBUG] AddPeer 原始响应: ', Response.ContentAsString);
      
      if Response.StatusCode = 200 then
      begin
        ResponseObj := TJSONObject.ParseJSONValue(Response.ContentAsString) as TJSONObject;
        try
          if Assigned(ResponseObj) then
          begin
            WriteLn('[DEBUG] JSON 解析成功');
            WriteLn('[DEBUG] 完整响应对象: ', ResponseObj.ToString);
            ResultObj := ResponseObj.GetValue('result') as TJSONObject;
            if Assigned(ResultObj) then
            begin
              WriteLn('[DEBUG] 找到 result 对象: ', ResultObj.ToString);
              var SuccessValue := ResultObj.GetValue('success');
              var MessageValue := ResultObj.GetValue('message');
              var PeerIDValue := ResultObj.GetValue('peerID');
              
              if Assigned(SuccessValue) then
                 WriteLn('[DEBUG] Success 字段: ', SuccessValue.ToString)
               else
                 WriteLn('[DEBUG] Success 字段: null');
               if Assigned(MessageValue) then
                 WriteLn('[DEBUG] Message 字段: ', MessageValue.ToString)
               else
                 WriteLn('[DEBUG] Message 字段: null');
               if Assigned(PeerIDValue) then
                 WriteLn('[DEBUG] PeerID 字段: ', PeerIDValue.ToString)
               else
                 WriteLn('[DEBUG] PeerID 字段: null');
              
              if Assigned(SuccessValue) and (SuccessValue is TJSONBool) then
                Result.Success := (SuccessValue as TJSONBool).AsBoolean
              else
                Result.Success := False;
                
              if Assigned(MessageValue) and (MessageValue is TJSONString) then
                Result.Message := (MessageValue as TJSONString).Value
              else
                Result.Message := 'Unknown response format';
                
              if Assigned(PeerIDValue) and (PeerIDValue is TJSONString) then
                Result.PeerID := (PeerIDValue as TJSONString).Value
              else
                Result.PeerID := '';
            end
            else
            begin
              WriteLn('[DEBUG] 未找到 result 对象');
              Result.Success := False;
              Result.Message := 'Invalid response format';
              Result.PeerID := '';
            end;
          end
          else
          begin
            WriteLn('[DEBUG] JSON 解析失败');
            Result.Success := False;
            Result.Message := 'Invalid JSON response';
            Result.PeerID := '';
          end;
        finally
          ResponseObj.Free;
        end;
      end
      else
      begin
        Result.Success := False;
        Result.Message := Format('HTTP Error: %d', [Response.StatusCode]);
        Result.PeerID := '';
      end;
    finally
      RequestObj.Free;
    end;
  finally
    StringStream.Free;
    HTTPClient.Free;
  end;
end;

function THyprspaceRPCClient.Status: TStatusReply;
var
  HTTPClient: THTTPClient;
  RequestObj: TJSONObject;
  RequestStr: string;
  StringStream: TStringStream;
  Response: IHTTPResponse;
  ResponseObj: TJSONObject;
  ResultObj: TJSONObject;
begin
  HTTPClient := THTTPClient.Create;
  StringStream := TStringStream.Create('', TEncoding.UTF8);
  try
    RequestObj := TJSONObject.Create;
    try
      RequestObj.AddPair('jsonrpc', '2.0');
      RequestObj.AddPair('method', 'status');
      RequestObj.AddPair('params', TJSONNull.Create);
      RequestObj.AddPair('id', TJSONNumber.Create(2));
      
      RequestStr := RequestObj.ToString;
      WriteLn('[DEBUG] Status 请求: ', RequestStr);
      StringStream.WriteString(RequestStr);
      StringStream.Position := 0;
      
      HTTPClient.ContentType := 'application/json';
      Response := HTTPClient.Post(FBaseURL, StringStream);
      
      WriteLn('[DEBUG] Status 响应状态码: ', Response.StatusCode);
      WriteLn('[DEBUG] Status 原始响应: ', Response.ContentAsString);
      
      if Response.StatusCode = 200 then
      begin
        ResponseObj := TJSONObject.ParseJSONValue(Response.ContentAsString) as TJSONObject;
        try
          if Assigned(ResponseObj) then
          begin
            WriteLn('[DEBUG] JSON 解析成功');
            WriteLn('[DEBUG] 完整响应对象: ', ResponseObj.ToString);
            ResultObj := ResponseObj.GetValue('result') as TJSONObject;
            if Assigned(ResultObj) then
            begin
              WriteLn('[DEBUG] 找到 result 对象: ', ResultObj.ToString);
              // Status 方法直接返回状态数据，不包装在 Success/Message 中
              var PeerIDValue := ResultObj.GetValue('peerID');
              var SwarmPeersValue := ResultObj.GetValue('swarmPeersCurrent');
              var NetPeersValue := ResultObj.GetValue('netPeersCurrent');
              var ListenAddrsValue := ResultObj.GetValue('listenAddrs');
              
              if Assigned(PeerIDValue) then
                WriteLn('[DEBUG] PeerID 字段: ', PeerIDValue.ToString)
              else
                WriteLn('[DEBUG] PeerID 字段: null');
              if Assigned(SwarmPeersValue) then
                WriteLn('[DEBUG] SwarmPeersCurrent 字段: ', SwarmPeersValue.ToString)
              else
                WriteLn('[DEBUG] SwarmPeersCurrent 字段: null');
              if Assigned(NetPeersValue) then
                WriteLn('[DEBUG] NetPeersCurrent 字段: ', NetPeersValue.ToString)
              else
                WriteLn('[DEBUG] NetPeersCurrent 字段: null');
              if Assigned(ListenAddrsValue) then
                WriteLn('[DEBUG] ListenAddrs 字段类型: ', ListenAddrsValue.ClassName)
              else
                WriteLn('[DEBUG] ListenAddrs 字段: null');
              
              Result.Success := True;
              Result.Message := 'Status retrieved successfully';
              
              if Assigned(PeerIDValue) and (PeerIDValue is TJSONString) then
                Result.PeerID := (PeerIDValue as TJSONString).Value
              else
                Result.PeerID := '';
                
              if Assigned(SwarmPeersValue) and (SwarmPeersValue is TJSONNumber) then
                Result.SwarmPeersCurrent := (SwarmPeersValue as TJSONNumber).AsInt
              else
                Result.SwarmPeersCurrent := 0;
                
              if Assigned(NetPeersValue) and (NetPeersValue is TJSONNumber) then
                Result.NetPeersCurrent := (NetPeersValue as TJSONNumber).AsInt
              else
                Result.NetPeersCurrent := 0;
                
              if Assigned(ListenAddrsValue) and (ListenAddrsValue is TJSONArray) then
                Result.Node := ListenAddrsValue as TJSONArray
              else
                Result.Node := nil;
            end
            else
            begin
              Result.Success := False;
              Result.Message := 'Invalid result format';
              Result.Node := nil;
            end;
          end
          else
          begin
            Result.Success := False;
            Result.Message := 'Invalid JSON response';
            Result.Node := nil;
          end;
        finally
          ResponseObj.Free;
        end;
      end
      else
      begin
        Result.Success := False;
        Result.Message := Format('HTTP Error: %d', [Response.StatusCode]);
        Result.Node := nil;
      end;
    finally
      RequestObj.Free;
    end;
  finally
    StringStream.Free;
    HTTPClient.Free;
  end;
end;

function THyprspaceRPCClient.Peers: TPeersReply;
var
  HTTPClient: THTTPClient;
  RequestObj: TJSONObject;
  RequestStr: string;
  StringStream: TStringStream;
  Response: IHTTPResponse;
  ResponseObj: TJSONObject;
  ResultObj: TJSONObject;
begin
  HTTPClient := THTTPClient.Create;
  StringStream := TStringStream.Create('', TEncoding.UTF8);
  try
    RequestObj := TJSONObject.Create;
    try
      RequestObj.AddPair('jsonrpc', '2.0');
      RequestObj.AddPair('method', 'peers');
      RequestObj.AddPair('params', TJSONNull.Create);
      RequestObj.AddPair('id', TJSONNumber.Create(3));
      
      RequestStr := RequestObj.ToString;
      WriteLn('[DEBUG] Peers 请求: ', RequestStr);
      StringStream.WriteString(RequestStr);
      StringStream.Position := 0;
      
      HTTPClient.ContentType := 'application/json';
      Response := HTTPClient.Post(FBaseURL, StringStream);
      
      WriteLn('[DEBUG] Peers 响应状态码: ', Response.StatusCode);
      WriteLn('[DEBUG] Peers 原始响应: ', Response.ContentAsString);
      
      if Response.StatusCode = 200 then
      begin
        ResponseObj := TJSONObject.ParseJSONValue(Response.ContentAsString) as TJSONObject;
        try
          if Assigned(ResponseObj) then
          begin
            WriteLn('[DEBUG] JSON 解析成功');
            ResultObj := ResponseObj.GetValue('result') as TJSONObject;
            if Assigned(ResultObj) then
            begin
              WriteLn('[DEBUG] 找到 result 对象: ', ResultObj.ToString);
              // Peers 方法直接返回对等节点数据
              var PeerAddrsValue := ResultObj.GetValue('peerAddrs');
              
              if Assigned(PeerAddrsValue) then
                WriteLn('[DEBUG] peerAddrs 类型: ', PeerAddrsValue.ClassName)
              else
                WriteLn('[DEBUG] peerAddrs 为空');
              
              Result.Success := True;
              Result.Message := 'Peers retrieved successfully';
                
              if Assigned(PeerAddrsValue) and (PeerAddrsValue is TJSONArray) then
              begin
                // 创建一个新的 TJSONArray 副本以避免内存访问问题
                Result.Peers := TJSONArray.Create;
                var SourceArray := PeerAddrsValue as TJSONArray;
                for var i := 0 to SourceArray.Count - 1 do
                begin
                  var ClonedValue := SourceArray.Items[i].Clone as TJSONValue;
                  Result.Peers.AddElement(ClonedValue);
                end;
              end
              else
                Result.Peers := nil;
            end
            else
            begin
              WriteLn('[DEBUG] 未找到 result 对象');
              Result.Success := False;
              Result.Message := 'Invalid result format';
              Result.Peers := nil;
            end;
          end
          else
          begin
            Result.Success := False;
            Result.Message := 'Invalid JSON response';
            Result.Peers := nil;
          end;
        finally
          ResponseObj.Free;
        end;
      end
      else
      begin
        Result.Success := False;
        Result.Message := Format('HTTP Error: %d', [Response.StatusCode]);
        Result.Peers := nil;
      end;
    finally
      RequestObj.Free;
    end;
  finally
    StringStream.Free;
    HTTPClient.Free;
  end;
end;

function THyprspaceRPCClient.GetNodeIP: TGetNodeIPReply;
var
  HTTPClient: THTTPClient;
  RequestObj: TJSONObject;
  RequestStr: string;
  StringStream: TStringStream;
  Response: IHTTPResponse;
  ResponseObj: TJSONObject;
  ResultObj: TJSONObject;
begin
  HTTPClient := THTTPClient.Create;
  StringStream := TStringStream.Create('', TEncoding.UTF8);
  try
    RequestObj := TJSONObject.Create;
    try
      RequestObj.AddPair('jsonrpc', '2.0');
      RequestObj.AddPair('method', 'nodeIp');
      RequestObj.AddPair('params', TJSONNull.Create);
      RequestObj.AddPair('id', TJSONNumber.Create(4));
      
      RequestStr := RequestObj.ToString;
      WriteLn('[DEBUG] GetNodeIP 请求: ', RequestStr);
      StringStream.WriteString(RequestStr);
      StringStream.Position := 0;
      
      HTTPClient.ContentType := 'application/json';
      Response := HTTPClient.Post(FBaseURL, StringStream);
      
      WriteLn('[DEBUG] GetNodeIP 响应状态码: ', Response.StatusCode);
      WriteLn('[DEBUG] GetNodeIP 原始响应: ', Response.ContentAsString);
      
      if Response.StatusCode = 200 then
      begin
        ResponseObj := TJSONObject.ParseJSONValue(Response.ContentAsString) as TJSONObject;
        try
          if Assigned(ResponseObj) then
          begin
            WriteLn('[DEBUG] JSON 解析成功');
            ResultObj := ResponseObj.GetValue('result') as TJSONObject;
            if Assigned(ResultObj) then
            begin
              WriteLn('[DEBUG] 找到 result 对象: ', ResultObj.ToString);
              // GetNodeIP 方法直接返回节点 IP 数据
              var NodeIPValue := ResultObj.GetValue('nodeIP');
              var AllIPsValue := ResultObj.GetValue('allIPs');
              var InterfaceValue := ResultObj.GetValue('interface');
              
              if Assigned(NodeIPValue) then
                WriteLn('[DEBUG] NodeIP 字段: ', NodeIPValue.ToString)
              else
                WriteLn('[DEBUG] NodeIP 字段: null');
              if Assigned(AllIPsValue) then
                WriteLn('[DEBUG] AllIPs 字段类型: ', AllIPsValue.ClassName)
              else
                WriteLn('[DEBUG] AllIPs 字段: null');
              if Assigned(InterfaceValue) then
                WriteLn('[DEBUG] Interface 字段: ', InterfaceValue.ToString)
              else
                WriteLn('[DEBUG] Interface 字段: null');
              
              Result.Success := True;
              Result.Message := 'Node IP retrieved successfully';
              
              if Assigned(NodeIPValue) and (NodeIPValue is TJSONString) then
                Result.NodeIP := (NodeIPValue as TJSONString).Value
              else
                Result.NodeIP := '';
                
              if Assigned(InterfaceValue) and (InterfaceValue is TJSONString) then
                Result.InterfaceName := (InterfaceValue as TJSONString).Value
              else
                Result.InterfaceName := '';
                
              if Assigned(AllIPsValue) and (AllIPsValue is TJSONArray) then
               begin
                 // 创建一个新的 TJSONArray 副本以避免内存访问问题
                 Result.AllIPs := TJSONArray.Create;
                 var SourceArray := AllIPsValue as TJSONArray;
                 for var j := 0 to SourceArray.Count - 1 do
                 begin
                   var ClonedValue := SourceArray.Items[j].Clone as TJSONValue;
                   Result.AllIPs.AddElement(ClonedValue);
                 end;
               end
              else
                Result.AllIPs := nil;
            end
            else
            begin
              WriteLn('[DEBUG] 未找到 result 对象');
              Result.Success := False;
              Result.Message := 'Invalid result format';
              Result.AllIPs := nil;
            end;
          end
          else
          begin
            Result.Success := False;
            Result.Message := 'Invalid JSON response';
            Result.AllIPs := nil;
          end;
        finally
          ResponseObj.Free;
        end;
      end
      else
      begin
        Result.Success := False;
        Result.Message := Format('HTTP Error: %d', [Response.StatusCode]);
        Result.AllIPs := nil;
      end;
    finally
      RequestObj.Free;
    end;
  finally
    StringStream.Free;
    HTTPClient.Free;
  end;
end;

// 主程序
var
  Client: THyprspaceRPCClient;
  Args: TAddPeerArgs;
  AddPeerReply: TAddPeerReply;
  StatusReply: TStatusReply;
  PeersReply: TPeersReply;
  I: Integer;

begin
  WriteLn('Mynetwork JSON-RPC 客户端示例');
  WriteLn('================================');
  WriteLn('');
  
  // 创建 RPC 客户端
  try
    Client := THyprspaceRPCClient.Create('mynetwork'); // 默认接口名
    WriteLn('成功连接到 JSON-RPC 服务器: ', Client.FBaseURL);
    WriteLn('');
  except
    on E: Exception do
    begin
      WriteLn('连接失败: ', E.Message);
      WriteLn('请确保 Mynetwork 服务正在运行');
      WriteLn('按任意键退出...');
      ReadLn;
      Exit;
    end;
  end;
  
  try
    // 1. 调用 Status 方法
    WriteLn('1. 获取节点状态...');
    StatusReply := Client.Status;
    WriteLn('  成功: ', StatusReply.Success);
    WriteLn('  消息: ', StatusReply.Message);
    if StatusReply.Success then
     begin
       WriteLn('  节点 ID: ', StatusReply.PeerID);
       WriteLn('  Swarm 对等节点数: ', StatusReply.SwarmPeersCurrent);
       WriteLn('  网络对等节点数: ', StatusReply.NetPeersCurrent);
       try
         if Assigned(StatusReply.Node) then
           WriteLn('  监听地址数量: ', StatusReply.Node.Count)
         else
           WriteLn('  监听地址: 无');
       except
         WriteLn('  监听地址: 解析错误');
       end;
     end;
    WriteLn('');
    
    // 2. 调用 Peers 方法
    WriteLn('2. 获取对等节点列表...');
    PeersReply := Client.Peers;
    WriteLn('  成功: ', PeersReply.Success);
    WriteLn('  消息: ', PeersReply.Message);
    try
       if Assigned(PeersReply.Peers) then
       begin
         try
           WriteLn('  对等节点数量: ', PeersReply.Peers.Count);
           for I := 0 to PeersReply.Peers.Count - 1 do
           begin
             try
               WriteLn('    [', I, '] ', PeersReply.Peers.Items[I].ToString);
             except
               WriteLn('    [', I, '] 解析错误');
             end;
           end;
         except
           WriteLn('  对等节点数量: 访问错误');
         end;
       end
       else
         WriteLn('  对等节点: 无');
     except
       WriteLn('  对等节点: 解析错误');
     end;
    WriteLn('');
    
    // 3. 调用 AddPeer 方法
    WriteLn('3. 添加新的对等节点...');
    Args.Name := 'dev';//'iZuf6i1b3ex8d5lek1xmqbZ';
    Args.ID := '12D3KooWK5Xm8D8tWRd52o3VWFZN2YYGYDzKACsc3ZjJ8PSeg8bV';//'12D3KooWMUWVCphVLL9vocZQw9cKtJoai3Tbg8zKYu2riCcE6xja';
    
    WriteLn('  节点名称: ', Args.Name);
    WriteLn('  节点 ID: ', Args.ID);
    
    AddPeerReply := Client.AddPeer(Args);
    WriteLn('  成功: ', AddPeerReply.Success);
    WriteLn('  消息: ', AddPeerReply.Message);
    WriteLn('  返回的 Peer ID: ', AddPeerReply.PeerID);
    
    // 4. 调用 GetNodeIP 方法
    WriteLn('4. 获取节点 IP 信息...');
    var GetNodeIPReply := Client.GetNodeIP;
    WriteLn('  成功: ', GetNodeIPReply.Success);
    WriteLn('  消息: ', GetNodeIPReply.Message);
    if GetNodeIPReply.Success then
    begin
      WriteLn('  节点主 IP: ', GetNodeIPReply.NodeIP);
      WriteLn('  网络接口: ', GetNodeIPReply.InterfaceName);
      if Assigned(GetNodeIPReply.AllIPs) then
       begin
         WriteLn('  所有 IP 地址:');
         for var k := 0 to GetNodeIPReply.AllIPs.Count - 1 do
         begin
           var IPValue := GetNodeIPReply.AllIPs.Items[k];
           if IPValue is TJSONString then
             WriteLn('    - ', (IPValue as TJSONString).Value)
           else
             WriteLn('    - ', IPValue.ToString);
         end;
       end
      else
        WriteLn('  所有 IP 地址: 无');
    end;
    WriteLn('');
    
    // 清理 GetNodeIP 返回的 AllIPs 数组
    if Assigned(GetNodeIPReply.AllIPs) then
      GetNodeIPReply.AllIPs.Free;
    
  finally
    Client.Free;
  end;
  
  WriteLn('');
  WriteLn('所有操作完成！');
  WriteLn('按任意键退出...');
  ReadLn;
end.
