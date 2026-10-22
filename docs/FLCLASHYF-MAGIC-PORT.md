# FlclashYF 魔改协议分析 & 移植到 mihomo

> 目标仓库：<https://github.com/llyufenggotest/FlclashYF>
> 移植目标：本仓库 `mihomo`（Alpha 分支）
> 验证样本：西贝订阅（110 个 `vless + xhttp + REALITY + uuid#x365` 节点）

---

## 1. 仓库结构与"APK 实际用的核心"

```
FlclashYF (Flutter 客户端)
├── core/mihomo            -> llyufenggotest/Clash.Meta-custom @ <分支>
│                              （FlClash 的 mihomo fork，再叠加机场自研协议）
└── core/sing-shadowsocks2 -> llyufenggotest/sing-shadowsocks2-meta @ flclash-yf
```

**重要**：`Clash.Meta-custom` 有 20+ 个分支，不同分支的私有协议实现差异很大。APK 实际使用的核心由 FlclashYF 的 submodule 指针决定：

| FlclashYF 分支 | 核心 commit |
|----------------|-------------|
| `integration/v0.9.6-yf`（最新 APK） | `0e8810d` |
| `main` | `e7dc72f` |
| `integration/flclash-yf` | `1396a82` |
| （旧的）`flclash-yf-core` 分支 | `bbf06cf` |

> 教训：只看 `flclash-yf-core`（对应 FlClash 0.8.127）会漏掉后续新增的关键行为——**`#x365` 节点的 User-Agent 注入就是 0e8810d 才加入的**。

---

## 2. 魔改清单（对齐 APK 核心 0e8810d）

| # | 模块 | 内容 | 是否需要移植 |
|---|------|------|--------------|
| 1 | `transport/xhttp`（Xray XHTTP 传输层） | 与上游 mihomo 逐字节相同（v1.19.22 起上游已内置） | 不需要 |
| 2 | VLESS `#x365` 线格式 | uuid 后缀剥除 + 请求头 `"X365"+0x01`、应答校验 5 字节 `"X365"` | **必须** |
| 3 | VLESS `#x365` 的 **UA 注入** | `adapter/outbound/vless.go`：`#x365` 节点若未指定 UA，强制注入 `Chrome/120.0.0.0 (Windows)`，否则服务端返回 `403 forbidden` | **必须（关键）** |
| 4 | VLESS `#blackstone` | 私有模式：`E`(bool)/`H`(hex) 选项，请求头追加 `md5(uuid)`(E 为真时) + H；响应可回传更新 H | 可选（`#blackstone` 节点用） |
| 5 | VLESS `#juzi` | 标准头后追加 8 字节 `HMAC-SHA256("hello_pidun", ver+uuid)` | 可选 |
| 6 | VLESS `#pure` | `0xA1` + 16B 原始 key，强制 TLS1.3 + ws + http/1.1，禁 UDP | 可选 |
| 7 | AnyTLS `#sl` | 密码带 `#sl` 时按 64 位 hex 直解（不走 sha256） | 可选 |
| 8 | 专有代理类型 `type: xhttp` / `type: Blackstone` | 机场闭源协议：API 拉动态节点（XOR+base64）、Blackstone ChaCha20-Poly1305 分块解密、证书 ASN.1 偏移内嵌配置、AES-CTR 自研 TCP 混淆、`device_id`/`device_name` 校验 | 可选（该类型订阅用） |
| 9 | `sing-shadowsocks2` 的 `#BLACKSTONE` 密码 | `pythonDeriveKey` 派生 + 自定义 wire | 配套 #8 |

### 关键陷阱一：UUID 会被"污染"

`common/utils/uuid.go` 的 `UUIDMap()` 对非法 UUID 会回退成 UUIDv5 派生值：

```
UUIDMap("84146b14-4a2c-4772-b24c-6ef6fd21499a")      = 84146b14-4a2c-4772-b24c-6ef6fd21499a
UUIDMap("84146b14-4a2c-4772-b24c-6ef6fd21499a#x365") = 035aee21-b935-5c8b-a733-d675de8cf76a  ← 错误
```

上游 mihomo 用这种 uuid 时 `-t` 能通过（不报错），但握手用的是派生出的错误 UUID → 鉴权失败。

### 关键陷阱二：`#x365` 节点必须带指定 UA

服务端校验 `User-Agent`。mihomo 的 xhttp 客户端默认注入 `Chrome/150` 的浏览器模拟头（`TryDefaultHeadersWith`），会被服务端拒绝：

```
resp = 403 Forbidden
body = "forbidden\n"        ← Go http.Error 风格
headers: X-Content-Type-Options: nosniff
```

APK 的做法（`adapter/outbound/vless.go`）：

```go
const x365UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func prepareX365XHTTPHeaders(headers map[string]string, x365 bool) map[string]string
// x365 为真且 headers 里没有 User-Agent 时，注入上面的 UA
```

---

## 3. 本次已落地的移植（本仓库已改）

| 文件 | 改动 |
|------|------|
| `transport/vless/vless.go` | `X365Marker`、`Client.IsX365`、`parseX365UUID()`、`IsX365UUID()`；`NewClient()` 先剥后缀再 `UUIDMap` |
| `transport/vless/conn.go` | `Conn.isX365`；`sendRequest()` 的 X365 分支；`recvResponse()` 的 X365 应答校验；`newConn()` 透传 |
| `adapter/outbound/vless.go` | `x365UserAgent` + `prepareX365XHTTPHeaders()`；`NewVless()` 计算 `isX365` 并作用于 xhttp `Headers` |
| `transport/vless/x365_test.go` | 标记解析 / UUID 不被污染 / 字节级线格式断言 |
| `adapter/outbound/x365_headers_test.go` | UA 注入：默认注入、显式 UA 优先、非 x365 不注入、不改原 map |

X365 线格式（与 APK 一致）：

```
请求:  'X' '3' '6' '5' 0x01 | cmd(1) | uuid(16) | [port(2) | atyp(1) | addr] | payload
应答:  'X' '3' '6' '5' <1 byte>      （只校验前 4 字节魔数）
```

---

## 4. 验证结果（全部实测）

| 验证项 | 方法 | 结果 |
|--------|------|------|
| 单元测试 | `go test ./transport/vless/ ./adapter/outbound/ -run X365` | 4 个测试全部 PASS |
| 编译 | `go build ./` | 成功 |
| 配置校验 | `mihomo -t -f 西贝订阅` | successful |
| 单节点出口 | `curl -x 127.0.0.1:17890 http://icanhazip.com` | `104.28.198.244`（≠ 本机 IP，流量确实出网） |
| 目标站连通 | gstatic / YouTube | 204 / 200 |
| **全量订阅** | 110 节点逐个 URLTest | **110 / 110 通过，平均延迟 670ms** |

典型延迟（与地理距离吻合）：香港 120ms、台湾 147ms、日本 202ms、立陶宛 430ms、德国 439ms、葡萄牙 540ms、俄罗斯 677ms。

---

## 5. 使用方式

配置**原样使用即可**，不需要手写 `User-Agent`（代码已自动注入）：

```yaml
proxies:
  - name: 俄罗斯01-X365
    type: vless
    server: bgp01.dnsefan04851.com
    port: 443
    uuid: 84146b14-4a2c-4772-b24c-6ef6fd21499a#x365   # ← 后缀即协议开关，不能删
    network: xhttp
    xhttp-opts:
      mode: auto
      path: /ru81
      headers:
        Host: dldir1.qq.com
    tls: true
    alpn: [h2, http/1.1]
    servername: dldir1.qq.com
    client-fingerprint: chrome
    reality-opts:
      public-key: j-nB37c22JoYowzSbRtceenR6nq9JoseOPgWOHf11zo
      short-id: c7cb949d076c9e93
    udp: true
    skip-cert-verify: false
```

注意：
* mihomo 需 ≥ v1.19.22（内置 `transport/xhttp`）。
* 若你的订阅里有 `global-client-fingerprint`（新版 mihomo 已移除）需删除，改为每个节点写 `client-fingerprint`。
* 标准 Xray 节点（无 `#x365`）不受影响：不会剥 uuid、不会注入 UA。

---

## 6. 尚未移植的可选特性（按需再做）

| 特性 | 移植位置 | 要点 |
|------|----------|------|
| `#blackstone` + `E`/`H` | `transport/vless/blackstone.go`（新增）、`conn.go`、`vless.go`、`adapter/outbound/vless.go` | 请求头追加 `md5(uuid)`(E) + H；响应可回传 1 字节长度 + H 更新；`NewClientWithBlackstoneOptions` |
| `#juzi` | `transport/vless/conn.go` | uuid 后 8 字节 `HMAC-SHA256("hello_pidun", ver+uuid)[:8]` |
| `#pure` | `transport/vless/pure.go` | `0xA1` + 16B key；限制 TLS1.3/ws/http-1.1，禁 UDP |
| AnyTLS `#sl` | `transport/anytls/client.go` | 密码 `#sl` 后缀 → 64 hex 直解 |
| `type: xhttp` / `Blackstone` 专有协议 | `adapter/outbound/xhttp.go`、`adapter/parser.go`、`constant/adapters.go` | 闭源 API 协议（XOR/base64 + ChaCha20-Poly1305 + AES-CTR TCP 混淆 + device_id/device_name） |
| `#BLACKSTONE` ss 密码 | `sing-shadowsocks2` 依赖 | 用 fork 版作为 `replace` 依赖 |
