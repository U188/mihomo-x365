# FlclashYF 魔改协议分析 & 全量移植到 mihomo

> 源仓库：<https://github.com/llyufenggotest/FlclashYF>
> 参考核心：FlclashYF `integration/v0.9.6-yf` 的 submodule 指针 → `Clash.Meta-custom@0e8810d`
> 目标：本仓库 `mihomo`（Alpha 分支）
> 验证样本：西贝订阅（110 个 `vless + xhttp + REALITY + uuid#x365` 节点）

---

## 1. 关键前提：对齐哪个提交

`Clash.Meta-custom` 有 20+ 个分支，私有协议实现差异极大：

| FlclashYF 分支 | 核心 commit | 说明 |
|----------------|-------------|------|
| `integration/v0.9.6-yf`（最新 APK） | `0e8810d` | **对齐基准** |
| `main` | `e7dc72f` | |
| `integration/flclash-yf` | `1396a82` | |
| （旧）`flclash-yf-core` | `bbf06cf` | 只有 x365，没有 UA 注入/blackstone |

只看旧分支会漏掉关键行为（`#x365` 的 User-Agent 注入就是 `0e8810d` 才加的）。

---

## 2. 完整魔改清单（相对上游 mihomo）

| # | 特性 | 位置 | 内容 |
|---|------|------|------|
| 1 | **`#x365` 线格式** | `transport/vless/{vless,conn}.go` | uuid 后缀 `#x365` → 请求 `'X''3''6''5' 0x01 | cmd | uuid | [port|atyp|addr] | payload`，应答校验 5 字节 `X365` |
| 2 | **`#x365` UA 注入** | `adapter/outbound/vless.go` | `#x365` 节点若未指定 UA，强制 `Chrome/120.0.0.0`；否则服务端回 `403 forbidden` |
| 3 | **`#blackstone` 私有模式** | `transport/vless/blackstone.go`、`transport/vless/conn.go` | `E`(bool)/`H`(hex) 选项；请求头追加 `md5(uuid)`(E 为真) + H；响应可回传 1 字节长度 + H 用于轮换 |
| 4 | **`transport/blackstone` 私有传输** | `transport/blackstone/{client,packet,uot}.go` | 独立加密传输（含 UOT） |
| 5 | **专有代理类型 `type: xhttp` / `type: Blackstone`** | `adapter/outbound/xhttp.go`、`blackstone_{selection,vless,error}.go`、`adapter/parser.go` | API 拉动态节点（XOR+base64）、Blackstone ChaCha20-Poly1305 分块解密、证书 ASN.1 偏移内嵌配置、AES-CTR 自研 TCP 混淆、`device_id`/`device_name` 校验、TCP ping 竞速选路、URLTest 拦截 |
| 6 | **`#juzi` 私有模式** | `transport/vless/conn.go` | 标准头后追加 8 字节 `HMAC-SHA256("hello_pidun", ver+uuid)` |
| 7 | **`#pure` 私有模式** | `transport/vless/pure.go`、`adapter/outbound/vless.go` | `0xA1` + 16B 原始 key；强制 TLS1.3 + ws + ALPN http/1.1；禁 UDP/XUDP |
| 8 | **`#sl`（闪连）AnyTLS 密码** | `transport/anytls/client.go`、`adapter/outbound/anytls.go` | 密码带 `#sl` 时按 64 位 hex 直解，不走 sha256 |
| 9 | **`#fastup` Trojan** | `adapter/outbound/trojan.go`、`adapter/parser.go` | 密码带 `#fastup` → `md5(password+mpw)`，并强制套 `h2mux`（sing-mux） |
| 10 | **`type: oppa` 私有协议** | `transport/oppa/{wire,packet}.go`、`adapter/outbound/oppa.go` | 独立协议 |
| 11 | **`#BLACKSTONE` ss 密码** | 依赖 `sing-shadowsocks2` | 密码后缀 → `pythonDeriveKey` 派生 + 自定义 wire |
| 12 | **链接转换扩展** | `common/convert/converter.go` | `xless://`（自动补 `#pure`）、`oppa://`、`wg://`、裸 JSON vmess、hysteria `mport`/`peer`、`udp=0`、`auto:` 前缀 |
| 13 | **UDP admission 预算** | `constant/udp_admission.go` | 专有适配器的 UDP 准入（`AdmitUDP`/`WithUDPAdmissionBudget`） |
| 14 | `constant/adapters.go` | — | 新增 `XHttp`（显示名 "Blackstone"）与 `Oppa` 适配器类型 |

### 两个必踩的坑

**坑一：UUID 被污染**

`common/utils/uuid.go` 的 `UUIDMap()` 对非法 UUID 会回退成 UUIDv5 派生值：

```
UUIDMap("84146b14-4a2c-4772-b24c-6ef6fd21499a")      = 84146b14-4a2c-4772-b24c-6ef6fd21499a
UUIDMap("84146b14-4a2c-4772-b24c-6ef6fd21499a#x365") = 035aee21-b935-5c8b-a733-d675de8cf76a  ← 错误
```

`mihomo -t` 不会报错，但握手用的是派生出的错误 UUID → 鉴权失败。

**坑二：`#x365` 必须带指定 UA**

服务端校验 `User-Agent`。默认的 `Chrome/150` 浏览器模拟头会被拒绝：

```
403 Forbidden
body: "forbidden\n"          ← Go http.Error 风格
X-Content-Type-Options: nosniff
```

---

## 3. 移植方式

以 `0e8810d` 为基准逐文件对齐：

* **纯新增文件** → 原样搬入（依赖仅限上游已有包）
* **上游更新的文件**（`constant/adapters.go`、`common/convert/converter.go`、`transport/anytls/session/*`）→ **只挑私有协议 hunk**，避免回退上游修复（如 `uniqueName` 去重、Smart 适配器、anytls session 新版逻辑）
* **`go.mod`** → 加 `replace github.com/metacubex/sing-shadowsocks2 => github.com/llyufenggotest/sing-shadowsocks2-meta v0.0.0-20261015095155-3e1762193a22`

### 已移植清单

| 类别 | 文件 |
|------|------|
| 新增 | `transport/blackstone/{client,packet,uot}.go`、`transport/oppa/{wire,packet}.go`、`transport/vless/{blackstone,pure}.go`、`adapter/outbound/{xhttp,blackstone_selection,blackstone_vless,blackstone_error,oppa}.go`、`constant/udp_admission.go` |
| 替换为 APK 版 | `transport/vless/{vless,conn}.go`、`adapter/outbound/{vless,trojan}.go` |
| 定点补丁 | `adapter/parser.go`（xhttp/Blackstone/oppa/fastup 分支）、`constant/adapters.go`（XHttp/Oppa 类型）、`transport/anytls/client.go` + `adapter/outbound/anytls.go`（`#sl`）、`common/convert/converter.go`（xless/oppa/wg 等，保留上游 `uniqueName`） |
| 依赖 | `go.mod` replace → fork 版 `sing-shadowsocks2`（`#BLACKSTONE`） |

### 未移植（与私有协议无关的 FlClash 平台特性）

`common/ebpf`、`component/smart`、`listener/sing_ebpf`、`common/probelimit`、`rules/provider` 内存优化、`hub/executor` 预检、`transport/snell` 的 OIX identity、`tunnel` 层 UDP admission 接线（`tunnel/udp_admission.go`）等。这些是 FlClash 客户端的平台能力，不影响协议互通。

---

## 4. 验证

| 项目 | 结果 |
|------|------|
| `go build ./...` | 通过 |
| `go test ./transport/vless ./transport/blackstone ./transport/oppa ./transport/anytls ./adapter ./adapter/outbound ./common/convert` | 全部 ok（含 APK 自带的 `blackstone_wire_test.go`、`x365_boundary_test.go`、`shanlian_password_test.go`、`xhttp_security_test.go` 等） |
| 单节点实连 | 出口 `104.28.198.244`（≠ 本机 IP）、gstatic `204`、YouTube `200` |
| **全量订阅** | **110 / 110 节点连通，平均 657ms** |

---

## 5. 使用

配置原样导入即可（`uuid` 上的 `#x365` 保留，不用手写 UA）：

```yaml
proxies:
  - name: 俄罗斯01-X365
    type: vless
    server: bgp01.dnsefan04851.com
    port: 443
    uuid: 84146b14-4a2c-4772-b24c-6ef6fd21499a#x365   # ← 协议开关
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
* mihomo ≥ v1.19.22（内置 `transport/xhttp`）
* 订阅里的 `global-client-fingerprint` 在新版 mihomo 已移除，需删除或改写到每个节点
* 标准 Xray 节点（无私有后缀）行为不变

二进制由 `.github/workflows/build-binaries.yml` 构建：windows/linux(amd64,arm64)/android-arm64/darwin(amd64,arm64)，打 `v*` tag 会自动发布 Release。
