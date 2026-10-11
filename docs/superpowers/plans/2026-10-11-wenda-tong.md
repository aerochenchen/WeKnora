# 问答通 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可单独使用的本地知识库问答软件「问答通」：本机入库、本机检索，页面作答调用 `llm.json` 里的 OpenAI 兼容接口。该接口可以在本机、内网或互联网。文书通若同时安装，只通过回环 HTTP 取摘录，公文仍由文书通自己的模型写成。

**Architecture:** 问答通是独立进程，不进文书通安装包，不与文书通共享数据库。进程是 WeKnora Lite（SQLite + 内存队列），编译时链入 anydoc；另起一个只监听 `127.0.0.1:50051` 的 docreader。向量模型在本机 Ollama `127.0.0.1:11434`。问答模型不随安装包分发，`knowledge-chat` 只请求 `llm.json` 里的那一个 OpenAI 兼容接口，地址可以是本机、内网或互联网。人打开 `http://127.0.0.1:8090` 即可问答。家目录、密钥、回环检查和问答接口地址放在 Go 启动器里，Linux / macOS / Windows 共用；`start.sh` 与 `start.ps1` 只负责调用它。文书通 Runtime 主进程另增三个只读工具，不调用问答通的问答接口。技能脚本没有网络，不能承担这次调用。

**Tech Stack:** WeKnora Lite（Go，`sqlite_fts5` + `anydoc`）· docreader（Python gRPC）· 本机 Ollama 仅作向量 · 可配置的 OpenAI 兼容对话 API · 文书通 Python Runtime（`/Users/chenzai/内部办公智能体`）

**Spec:** 本文件即方案。决策来自对 WeKnora v0.8.2 与文书通本地部署版（Office Agent Workbench，local 档位）的对照，不另写规格。

## Global Constraints

- 产品名是「问答通」。进程、数据目录、配置文件都用这个名字，不在界面上把腾讯云、微信渠道或 WeKnora Cloud 当作必选项。
- 只监听 `127.0.0.1`。`config/config.yaml` 默认 `server.host` 是 `0.0.0.0`。`LoadConfig` 把文件读成字符串再 `Unmarshal`，点号环境变量不会覆盖已经写在 YAML 里的值。问答通必须自带一份把 `host` 写成 `127.0.0.1` 的配置，不能只设 `SERVER_HOST`。
- 端口：问答通 API `127.0.0.1:8090`（避开文书通 Runtime 的 `127.0.0.1:8765`）。docreader `127.0.0.1:50051`。Ollama 只给向量模型，`127.0.0.1:11434`。
- 安装介质、docreader 镜像、向量模型权重离线交付并带 SHA256。禁止在目标机执行 `docker compose pull`、`ollama pull`、检查更新。问答请求可以出网，只要 `llm.json` 里的地址在互联网上。
- 不使用 WeKnora Cloud、MinerU Cloud、PaddleOCR 云端、腾讯云 COS、腾讯云向量库、联网搜索、IM 渠道。
- 问答通自己的页面走 `POST /api/v1/knowledge-chat/:session_id`。该接口里的对话模型只有一条调用路径：`llm.json` 里的地址。文书通不调用该接口，也不调用 MCP 的 `ask`，只调用 `knowledge-search` 与分块阅读。
- 问答模型：`source` 为 `remote`，`baseUrl` 与 `apiKey`、`modelName` 来自本机 `~/.wenda-tong/llm.json`，不写进仓库。`baseUrl` 可以是本机（如 `http://127.0.0.1:8000/v1`）、内网（如 `http://10.0.0.8:8000/v1`）或互联网（如 `https://api.openai.com/v1`）。只拒绝空地址，以及不是 `http` / `https` 的地址。不按网段或域名名单拒绝。只有这一个对话接口：它失败时不改连另一个主机，也不改用 Ollama 做对话。向量模型仍是本机 Ollama。
- 向量模型仍在本机 Ollama：`bge-m3`，维度 1024。启动器发现 Ollama 里没有该模型就退出，不执行 `ollama pull`。
- 数据目录只用 `os.UserHomeDir()`：Unix 是 `~/.wenda-tong`，Windows 是 `%USERPROFILE%\.wenda-tong`。这与文书通 Python `Path.home() / ".wenda-tong"` 是同一目录。文书通和问答通必须是同一操作系统上的两个进程。
- 家目录解析、`crypto/rand` 生成密钥、文件访问控制、回环检查、问答接口地址是否为空、Ollama 里是否已有 `bge-m3`，只写在 Go 启动器里。`start.sh` 与 `start.ps1` 把参数原样转给启动器，不各自实现这些检查。
- 密钥文件在 Unix 上是 `0600`。在 Windows 上把 DACL 收成仅当前用户，不用 `chmod` 充当访问控制。生成密钥用 `crypto/rand`，目标机不需要 `openssl`。
- 确认端口时向 `127.0.0.1` 拨号，并用 `docker port` 核对 docreader 发布在 `127.0.0.1:50051`。不把 `lsof` 当作验收命令。
- v1 的 Windows 构建目标是 amd64。anydoc 使用 `x86_64-pc-windows-gnu` 的 `libanydoc_go.a`，与桌面版 CGO 的 MinGW 一致。`x86_64-pc-windows-msvc` 的 `anydoc_go.lib` 不放到这条链接会搜到的目录。
- 执行顺序：Task 1 → Task 8 → Task 9 → Task 2 至 Task 7。Task 2、Task 3 不要先写一套 bash 逻辑再搬进启动器。
- 给文书通看的检索结果是资料线索，不是口径正本。口径、范文、红线仍留在文书通自己的知识库。问答通页面上的回答同样只依据库内摘录，不把摘录写成制度。
- 单文件沿用 `MAX_FILE_SIZE_MB=50`。空间配额沿用默认 10GB（含原文件、文本、向量、索引）。不新造篇数上限。
- 文书通 local 档位：技能脚本拒绝网络（macOS `deny network*`，Windows AppContainer 无网络能力）。调用只能写在 Runtime 主进程。
- 仅在用户明确要求时 commit。

---

## 不做

- 不把 WeKnora 源码嵌进文书通，不共享 SQLite。
- 文书通不调用 `POST /api/v1/knowledge-chat` 或 MCP 的 `ask`。问答通自己的页面可以调用 `knowledge-chat`。
- 不把 PyTorch、向量模型或对话模型打进文书通标准包。问答模型不在本机加载。4GB 工位默认不启动问答通，因为 docreader 和本机向量模型仍占内存。
- v1 不做音频（mp3/wav 等），不做视频。
- v1 图片能入库；图中文字可检索放到 v2（本机视觉模型）。v1 不因此阻塞。
- 不在问答通里再装一个本地对话大模型。
- 不把问答通放进 WSL 来充当 Windows 版。WSL 家目录是 `/home/<用户>`，与 Windows 上的 `%USERPROFILE%\.wenda-tong` 不是同一个目录，文书通读不到 `config.json`。
- 不把问答通打进 WeKnora Lite 的 Wails / NSIS 安装包。页面仍用浏览器打开 `http://127.0.0.1:8090`。
- v1 不做 Windows ARM。`scripts/build-anydoc-lib.sh` 遇到该目标继续失败。

## 文件与格式分工

| 后缀 | v1 解析 |
|---|---|
| txt、md、markdown、json、csv | Go `SimpleFormatReader`，无额外服务 |
| png、jpg、jpeg、gif | Go 收进库。v1 不承诺按图中文字检索 |
| doc、docx、ppt、pptx、xls、xlsx、epub | 进程内 anydoc。必须 `-tags anydoc` |
| pdf、html、htm、mhtml、xmind | 本机 docreader。PDF 不要交给 anydoc |

白名单以 `internal/utils/import_filetypes.go` 的 `supportedImportFileExtensions` 为准。不要为了这张表去改白名单。

## File Structure

问答通侧（本仓库）：

```
cmd/wenda-tong/          # 启动器。三个系统共用家目录、密钥、回环、问答接口地址
deploy/wenda-tong/
  config.yaml          # 从 config/config.yaml 复制，只改监听与关闭外联
  env                  # Lite 运行环境；家目录两项由启动器覆盖，不留给 shell 展开
  start.sh             # exec 同一目录下的启动器，参数原样传递
  start.ps1            # 调用同一目录下的 wenda-tong.exe，参数原样传递
Makefile               # build-lite 的 -tags 加入 anydoc（见 Task 1）
```

文书通侧（`/Users/chenzai/内部办公智能体`，本方案不在该仓库自动改代码）：

```
runtime/src/office_agent/corpus_client.py          # 新建
runtime/src/office_agent/tools.py                  # 注册三个工具
runtime/src/office_agent/agent_loop.py             # TOOL_LABELS / TOOL_SCHEMAS / 系统提示一句
runtime/src/office_agent/permissions.py            # 回环才允许，写审计，不进技能脚本
runtime/tests/test_corpus_client.py                # 新建
```

运行时数据（不进 git）。下面的 `~` 在 Windows 上是 `%USERPROFILE%`，由启动器用 `os.UserHomeDir()` 解析，不由 shell 展开：

```
~/.wenda-tong/llm.json         # 问答模型 base_url、api_key、model_name；本机、内网、互联网均可；仅当前用户可读
~/.wenda-tong/config.json      # base_url、api_key、knowledge_base_id
~/.wenda-tong/data/            # SQLite 与原文件
```

`llm.json` 示例（地址按实际接口填写，本机、内网、互联网均可）：

```json
{
  "base_url": "http://10.0.0.8:8000/v1",
  "api_key": "<单位发放的钥匙>",
  "model_name": "<单位模型名>"
}
```

---

### Task 1: 本机只能从回环访问的 Lite 构建

**Files:**
- Modify: `Makefile:313`
- Create: `deploy/wenda-tong/config.yaml`
- Create: `deploy/wenda-tong/env`
- Create: `deploy/wenda-tong/start.sh`
- Create: `deploy/wenda-tong/start.ps1`

**Interfaces:**
- Consumes: 现有 `make build-lite`、`make anydoc-lib`、`.env.lite.example`
- Produces: 二进制 `WeKnora-lite`，监听 `127.0.0.1:8090`，`GET /health` 返回成功

`Makefile` 里 `build-lite` 现在写死 `-tags "sqlite_fts5"`，不读 `GO_BUILD_TAGS`。改成：

```makefile
go build -tags "sqlite_fts5 anydoc" -ldflags="-w -s $$LDFLAGS" -o $(BINARY_NAME)-lite $(MAIN_PATH)
```

`build-lite` 目标开头增加依赖 `anydoc-lib`。没有 Rust 工具链时构建必须失败，不能静默编出没有 anydoc 的二进制。

`deploy/wenda-tong/config.yaml` 从 `config/config.yaml` 复制后只改这两处，其余检索阈值先不动：

```yaml
server:
  port: 8090
  host: "127.0.0.1"
```

并把 `conversation.enable_rerank` 设为 `false`。v1 不部署重排模型，避免检索被空的重排配置卡住。

`deploy/wenda-tong/env` 基于 `.env.lite.example`，至少包含：

```bash
GIN_MODE=release
DB_DRIVER=sqlite
DB_PATH=
RETRIEVE_DRIVER=sqlite
STORAGE_TYPE=local
LOCAL_STORAGE_BASE_DIR=
STREAM_MANAGER_TYPE=memory
OLLAMA_BASE_URL=http://127.0.0.1:11434
DOCREADER_ADDR=127.0.0.1:50051
NEO4J_ENABLE=false
WEKNORA_SANDBOX_MODE=disabled
ENABLE_GRAPH_RAG=false
MAX_FILE_SIZE_MB=50
SSRF_DNS_WHITELIST_ONLY=false
```

`SSRF_DNS_WHITELIST_ONLY` 保持关闭。WeKnora 默认会拒绝发往回环和内网的 URL。启动器在初始化时把 `llm.json` 的主机和 `127.0.0.1` 写入 `SSRF_WHITELIST_EXTRA`，本机与内网的问答接口、以及本机 Ollama 才能通过 `ValidateURLForSSRF`。互联网上的 `https` 地址按默认规则放行，不必写入白名单。不要再用域名名单拒绝公网模型。

`DB_PATH` 与 `LOCAL_STORAGE_BASE_DIR` 留空。启动器在读完 `env` 之后覆盖这两项，分别指向 `<UserHomeDir>/.wenda-tong/data/weknora.db` 与 `<UserHomeDir>/.wenda-tong/data/files`。环境里若仍残留字面量 `$HOME`，启动器退出。

`SYSTEM_AES_KEY` 与 `JWT_SECRET` 首次启动时由启动器用 `crypto/rand` 生成一次，写入数据目录，不写进仓库。AES 密钥 16 字节的十六进制，JWT 密钥 32 字节的十六进制。

`start.sh` 与 `start.ps1` 只调用同一安装目录下的启动器并原样传递参数。工作目录、`server.host` 检查和密钥生成在 Task 8，不写在脚本里。

- [ ] **Step 1:** 改 `Makefile` 的 `build-lite` tags，并让它依赖 `anydoc-lib`
- [ ] **Step 2:** 写入 `deploy/wenda-tong/config.yaml` 与 `env`。`start.sh` 和 `start.ps1` 的正文以 Task 8 为准，这里只要求它们存在且不包含检查逻辑
- [ ] **Step 3:** 在有 Rust 的开发机执行 `make build-lite`
- [ ] **Step 4:** 用这份配置启动后执行 `curl -sS http://127.0.0.1:8090/health`，期望 HTTP 200。再执行 `curl -sS --connect-timeout 2 http://<本机局域网 IP>:8090/health`，期望连接失败

---

### Task 2: 本机 docreader 只解析 PDF、网页和 XMind

**Files:**
- Modify: `cmd/wenda-tong/`（子命令 `docreader`，见 Task 8）
- Create: `deploy/wenda-tong/docreader.sh`（转给启动器）

**Interfaces:**
- Consumes: `docker/Dockerfile.docreader`，环境变量 `DOCREADER_ADDR`
- Produces: `127.0.0.1:50051` 上的 gRPC 文档解析。不映射到 `0.0.0.0`

启动器使用已经离线导入的镜像，发布端口写成 `127.0.0.1:50051:50051`。不要使用 `ports: ["50051:50051"]` 这种会绑全部网卡的写法。Windows 上这是 Docker Desktop 的 Linux 引擎，不是 Windows 容器。

开发机可以构建镜像；交付给内网时交付镜像 tar 和 SHA256，由 `docker load` 导入。本地没有该镜像时退出，不尝试 pull。

主进程起来之前，启动器对 `127.0.0.1:50051` 做一次 TCP 连接，失败则退出。启动后 `docker port <容器> 50051` 的结果必须是 `127.0.0.1:50051`。

- [ ] **Step 1:** 验收 Task 8 的 `docreader`：发布参数含 `127.0.0.1:50051:50051`，本地无镜像时退出且没有 `docker pull`
- [ ] **Step 2:** 验收 `start`：`127.0.0.1:50051` 连不上时主进程不起来
- [ ] **Step 3:** 镜像已加载时，`docker port` 的地址必须是 `127.0.0.1:50051`

---

### Task 3: 问答模型、本机向量模型和只读资料库

**Files:**
- Modify: `cmd/wenda-tong/`（子命令 `bootstrap`，见 Task 8）
- Create: `deploy/wenda-tong/bootstrap.sh`（转给启动器）

**Interfaces:**
- Consumes: `<UserHomeDir>/.wenda-tong/llm.json`；`POST /api/v1/initialization/initialize/:kbId`；本机 Ollama 仅检查 `bge-m3`
- Produces: `<UserHomeDir>/.wenda-tong/config.json`

```json
{
  "base_url": "http://127.0.0.1:8090",
  "api_key": "<仅 retrieve 能力>",
  "knowledge_base_id": "<kb id>"
}
```

文件仅当前用户可读：Unix 模式 `0600`，Windows 为仅当前用户的 DACL。交给文书通的 API Key 只授予检索（`retrieve`），不授予 `chat` 或写入。人使用问答通页面时走登录后的 JWT，不使用这把钥匙。入库和页面问答都由这个本机账号完成。

启动器读取 `llm.json`。`base_url` 为空，或协议不是 `http` / `https` 时，非 0 退出，不创建知识库。`http://127.0.0.1:8000/v1`、`http://10.0.0.8:8000/v1`、`https://api.openai.com/v1` 都允许。然后请求 `GET http://127.0.0.1:11434/api/tags`，没有 `bge-m3` 就退出，不下载。通过后把问答接口主机与 `127.0.0.1` 写入 `SSRF_WHITELIST_EXTRA`，创建知识库「问答资料」，并调用：

```bash
POST /api/v1/initialization/initialize/$KB_ID
```

```json
{
  "llm": {
    "source": "remote",
    "modelName": "<llm.json 的 model_name>",
    "baseUrl": "<llm.json 的 base_url>",
    "apiKey": "<llm.json 的 api_key>"
  },
  "embedding": {
    "source": "local",
    "modelName": "bge-m3",
    "baseUrl": "http://127.0.0.1:11434",
    "dimension": 1024
  },
  "rerank": {"enabled": false},
  "multimodal": {"enabled": false},
  "documentSplitting": {"chunkSize": 512, "chunkOverlap": 80, "separators": ["\n\n", "\n", "。"]},
  "nodeExtract": {"enabled": false},
  "questionGeneration": {"enabled": false}
}
```

解析引擎留默认：简单格式走 Go，Office/EPUB 走 anydoc，PDF/HTML/XMind 走 docreader。不开启联网搜索。

- [ ] **Step 1:** 验收 Task 8 的 `bootstrap`。`base_url` 为空、协议不是 `http`/`https`，或缺少 `bge-m3` 时没有初始化请求。`bootstrap.sh` 只转发到启动器
- [ ] **Step 2:** `http://127.0.0.1:8000/v1`、`http://10.0.0.8:8000/v1`、`https://api.openai.com/v1` 都允许进入初始化。这条用 `go test` 覆盖，不依赖 bash
- [ ] **Step 3:** 用文书通那把 API Key 调用 `POST /api/v1/knowledge-chat/:session_id` 与上传接口，期望 403；调用 `POST /api/v1/knowledge-search` 期望不是 401/403
- [ ] **Step 4:** 用页面账号的 JWT 调用 `GET /api/v1/initialization/config/:kbId`，确认对话模型 `source` 为 `remote` 且 `baseUrl` 等于 `llm.json`，向量模型为 `bge-m3` 且地址是 `127.0.0.1:11434`

---

### Task 4: 规定格式能入库并被检索

**Files:**
- Create: `deploy/wenda-tong/testdata/`（每种后缀一份小样例，不提交真实公文）
- Create: `deploy/wenda-tong/smoke.sh`

**Interfaces:**
- Consumes: `POST /api/v1/knowledge-bases/:id/knowledge/file`（管理员令牌，不是文书通那把 retrieve key）
- Produces: 样例里的一句独特原文能被 `POST /api/v1/knowledge-search` 命中

样例各一份即可：`txt`、`md`、`json`、`csv`、`docx`、`pptx`、`xlsx`、`pdf`（带文字层）、`html`、`xmind`、`epub`、`png`。`doc` 与 `ppt`、`xls` 用各一份旧格式。`mhtml` 用一份单页归档。

`smoke.sh` 对每个文件上传，轮询该知识条目直到 `parse_status=completed` 或失败。文本类、Office、PDF、HTML、XMind、EPUB 必须在返回的 `content` 里看到样例里预埋的唯一句子。`png` 只要求 `parse_status=completed`，v1 不要求句子出现。

请求体：

```json
{
  "query": "<样例里的唯一句子>",
  "knowledge_base_ids": ["<kb id>"],
  "match_count": 5,
  "rerank": {"enabled": false}
}
```

检索这一步只打本机向量模型，不调用问答模型。

- [ ] **Step 1:** 准备不含真实公文的样例
- [ ] **Step 2:** 跑 `smoke.sh`
- [ ] **Step 3:** 文本、Office、PDF、网页、XMind、EPUB 全部命中；png 完成解析；mp4 上传被拒绝

---

### Task 5: 不启动文书通，页面经配置的问答接口独立问答

**Files:**
- Modify: `deploy/wenda-tong/smoke.sh`

**Interfaces:**
- Consumes: Task 3 配置的问答接口，Task 4 已入库的 txt 样例
- Produces: 一条带摘录依据的回答。此时文书通进程不存在

用页面账号的 JWT，不用文书通的 API Key：

```bash
POST /api/v1/sessions
{"title": "独立问答"}

POST /api/v1/knowledge-chat/$SESSION_ID
{"query": "<样例里的唯一句子问成一个问题>", "knowledge_base_ids": ["<kb id>"], "web_search_enabled": false}
```

`knowledge-chat` 是 SSE。冒烟在流里收集回答文本，必须同时满足：回答非空；回答中出现样例里的唯一句子或文档标题；对话请求的主机等于 `llm.json` 的主机。停掉该接口时期望失败，请求不得改发到另一个主机。向量请求仍可以打到 `127.0.0.1:11434`。

浏览器验收：只开问答通、本机 Ollama 和 `llm.json` 里的问答接口，打开 `http://127.0.0.1:8090`，对同一份 txt 提问，页面上能看到回答和出处。不启动文书通。

- [ ] **Step 1:** 在 `smoke.sh` 增加上述会话与 `knowledge-chat` 调用
- [ ] **Step 2:** 文书通未运行、`llm.json` 里的问答接口可用时跑通 txt 这一条
- [ ] **Step 3:** 停掉该接口后同一条必须失败，且没有发往其他主机的对话请求
- [ ] **Step 4:** 在浏览器打开 `http://127.0.0.1:8090` 复现同一次提问

---

### Task 6: 文书通主进程只读调用

**Files:**
- Create: `/Users/chenzai/内部办公智能体/runtime/src/office_agent/corpus_client.py`
- Create: `/Users/chenzai/内部办公智能体/runtime/tests/test_corpus_client.py`
- Modify: `/Users/chenzai/内部办公智能体/runtime/src/office_agent/tools.py`
- Modify: `/Users/chenzai/内部办公智能体/runtime/src/office_agent/agent_loop.py`
- Modify: `/Users/chenzai/内部办公智能体/runtime/src/office_agent/permissions.py`

**Interfaces:**
- Consumes: `Path.home() / ".wenda-tong" / "config.json"`；`POST /api/v1/knowledge-search`；`GET /api/v1/chunks/:knowledge_id`
- Produces: 工具 `corpus_health`、`corpus_search`、`corpus_read`

`load_config` 的默认路径是 `Path.home() / ".wenda-tong" / "config.json"`。不要写死 `~`、`/home` 或盘符。在 Windows 上这就是 `%USERPROFILE%\.wenda-tong\config.json`，与启动器的 `os.UserHomeDir()` 相同。

`corpus_client.py`：

```python
def load_config(path: Path) -> CorpusConfig: ...
def health(cfg: CorpusConfig, timeout: float = 2.0) -> bool: ...
def search(cfg: CorpusConfig, query: str, limit: int = 5) -> list[CorpusHit]: ...
def read(cfg: CorpusConfig, knowledge_id: str, limit: int = 20) -> str: ...
```

`CorpusHit` 字段：`knowledge_id`、`title`、`content`、`score`。`base_url` 的主机不是 `127.0.0.1` 或 `localhost` 时，三个函数都拒绝，不发请求。

`search` 的 HTTP 体与 Task 4 相同，并带 `X-API-Key`。`read` 调用 `GET /api/v1/chunks/{knowledge_id}`，把分块正文按顺序拼成一段，总长度截断到 8000 字。

`tools.py` 的 `handlers` 增加这三项，实现放在 `ToolExecutor` 上。问答通没启动时 `corpus_health` 返回 `{"ok": false}`，另外两个返回 `{"ok": false, "error": "问答通未运行"}`，不抛给模型假装已查过。

`agent_loop.py` 的 `TOOL_LABELS` 与 `TOOL_SCHEMAS` 增加同名工具。描述写明：查的是长期资料库，不是当前工作区；结果是资料线索，不能写成制度或口径；当前文件夹里的文件仍用 `workspace_read` / `workspace_extract`。系统提示增加一句：资料库不可用时不得声称已经查过资料。

`permissions.py`：这三个工具不放进 `RISKY_TOOLS`。它们在 `execute` 里单独审计（工具名、是否成功、命中条数或 knowledge_id），不把正文写入审计。主机不是回环则 `PermissionDenied`。

测试用本机假 HTTP 服务，不依赖真实问答通：

- 配置指向 `10.1.1.1` 时 `search` 拒绝且没有出站
- 假服务返回一条 `SearchResult` 时 `search` 映射出 `title` 与 `content`
- 连接被拒绝时 `health` 为 False

- [ ] **Step 1:** 写失败测试并确认失败
- [ ] **Step 2:** 实现 `corpus_client.py` 与三个工具
- [ ] **Step 3:** 在文书通仓库运行 `pytest runtime/tests/test_corpus_client.py`
- [ ] **Step 4:** 问答通与文书通同时运行时，让模型调用 `corpus_search`，审计里有工具名、没有全文

---

### Task 7: 离线交付与和文书通安装包分开

**Files:**
- Create: `deploy/wenda-tong/PACKAGE.txt`

**Interfaces:**
- Consumes: Task 1 到 Task 6 的产物
- Produces: 一份不改文书通 `setup.exe` 的介质清单

`PACKAGE.txt` 按平台列出文件，SHA256 由打包脚本填入：该平台的 Lite 二进制、该平台的启动器、docreader 镜像 tar、`bge-m3` 权重、`PACKAGE.txt` 自身。Windows amd64 的二进制是 `WeKnora-lite.exe` 与 `wenda-tong.exe`。不包含对话模型权重，也不包含 Docker Desktop 或 Ollama 的安装包；这两样由使用单位离线安装，Windows 上 Docker 必须是 Linux 容器引擎。内网模型地址和钥匙由使用单位写在数据目录的 `llm.json`，不放进介质。问答接口若在互联网上，运行时需要出网；安装介质本身仍离线。说明使用单位保管介质，安装前核对哈希。文书通本地部署包不重新打包、不重新送审。

- [ ] **Step 1:** 写清单模板和打包脚本的输出路径 `dist/wenda-tong/`
- [ ] **Step 2:** 问答接口仍可达时按清单启动，重复 Task 5 的独立问答，以及 Task 4 的 pdf 检索。接口在本机或内网时，切断互联网后这两项仍成功

---

### Task 8: 三个系统共用的 Go 启动器

**Files:**
- Create: `cmd/wenda-tong/`
- Modify: `deploy/wenda-tong/start.sh`
- Create: `deploy/wenda-tong/start.ps1`
- Create: `deploy/wenda-tong/docreader.sh`
- Create: `deploy/wenda-tong/bootstrap.sh`
- Test: `cmd/wenda-tong/` 下的 `go test`

**Interfaces:**
- Consumes: `deploy/wenda-tong/env`、`deploy/wenda-tong/config.yaml`、`os.UserHomeDir()`、本机 Docker、`http://127.0.0.1:11434/api/tags`
- Produces: 子命令 `start`、`docreader`、`bootstrap`。安装目录里启动器与 `WeKnora-lite` 并列；Windows 上文件名是 `wenda-tong.exe`

启动器是这些检查的唯一实现。脚本内容只有调用同一目录下的启动器并原样传递参数：

```bash
exec "$(dirname "$0")/wenda-tong" "$@"
```

```powershell
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "wenda-tong.exe") @args
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
```

`start`：

- 用 `os.UserHomeDir()` 得到数据目录。Unix 与 Windows 都是该目录下的 `.wenda-tong`。解析失败时退出。
- 读 `env` 后覆盖 `DB_PATH` 与 `LOCAL_STORAGE_BASE_DIR`。发现值里还有字面量 `$HOME` 时退出。
- 读 `config.yaml`，`server.host` 不是 `127.0.0.1` 时退出。
- 数据目录里没有 AES 与 JWT 密钥时，用 `crypto/rand` 各生成一次并写入。写完后收紧访问：Unix `os.Chmod(0600)`；Windows 用 `golang.org/x/sys/windows` 把 DACL 设为仅当前用户。两种系统都做，Windows 上的 `chmod` 不算完成。
- 对 `127.0.0.1:50051` 做一次 TCP 连接，失败则不启动主进程。
- 工作目录能够读到 `deploy/wenda-tong/config.yaml`，然后执行同目录的 Lite 二进制。

`docreader`：本地没有指定镜像时退出，不执行 `docker pull`。运行参数包含 `-p 127.0.0.1:50051:50051`。之后 `docker port` 的对应行必须是 `127.0.0.1:50051`，否则退出并停掉刚拉起的容器。

`bootstrap`：读 `llm.json`。`base_url` 为空或协议不是 `http` / `https` 时退出，且不调用初始化接口。本机、内网、互联网地址都接受。然后检查 Ollama 已有 `bge-m3`，没有则退出，不下载。通过后把问答接口的主机和 `127.0.0.1` 写入 `SSRF_WHITELIST_EXTRA`，按 Task 3 创建知识库并写 `config.json`，写完按上面的规则收紧权限。

- [ ] **Step 1:** 为家目录、字面量 `$HOME`、空的 `base_url`、非 `http`/`https` 的 `base_url`、非回环 `server.host` 写失败测试。另写本机、内网、互联网三个地址都能通过的测试
- [ ] **Step 2:** 实现三个子命令和两个薄脚本
- [ ] **Step 3:** `go test ./cmd/wenda-tong/` 通过。`https://api.openai.com/v1` 允许初始化；空地址不允许，且没有初始化请求
- [ ] **Step 4:** 在当前开发机上，`start.sh bootstrap` 写出的 `config.json` 权限是 `0600`，路径在 `os.UserHomeDir()` 下

---

### Task 9: Windows amd64 链入 anydoc 并进入 CLI 发行矩阵

**Files:**
- Modify: `scripts/build-anydoc-lib.sh`
- Modify: `third_party/anydoc-go/anydoc.go`（仅当 MinGW 链接缺系统库时补 LDFLAGS）
- Modify: `.github/workflows/release-lite.yml` 的 `build-binary`

**Interfaces:**
- Consumes: 现有 `#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/lib/windows_amd64 -lanydoc_go`；`release-lite.yml` 里已有的 `CGO_ENABLED=1`
- Produces: `third_party/anydoc-go/lib/windows_amd64/libanydoc_go.a`，以及带 `sqlite_fts5 anydoc` 的 `WeKnora-lite.exe`

桌面版在 `windows-latest` 上用 MinGW 做 CGO。`build-anydoc-lib.sh` 今天把 `x86_64-pc-windows-msvc` 的 `anydoc_go.lib` 放进 `lib/windows_amd64`，MinGW 的 `-lanydoc_go` 链不上这个文件。

改为：

- 增加目标 `x86_64-pc-windows-gnu`，产物名 `libanydoc_go.a`，目录仍是 `lib/windows_amd64`。
- `x86_64-pc-windows-msvc` 若保留，产物改放到 `lib/windows_amd64_msvc`，避免和 MinGW 搜库目录混在一起。
- 在 Windows 上不传 `TARGET` 时，默认编 gnu 目标，因为本仓库的 Windows CGO 用的是 gcc。

`release-lite.yml` 的 `build-binary` 增加 `windows-latest` / `windows` / `amd64`。这一档安装 Rust 与 MinGW，先跑 `scripts/build-anydoc-lib.sh`，再 `go build -tags "sqlite_fts5 anydoc"`。`windows-latest` 的默认 shell 是 PowerShell，现有构建和打包步骤要显式 `shell: bash`，或为这一档单独写打包；产物名 `WeKnora-lite_*_windows_amd64`。Linux 与 macOS 两档也先跑 `build-anydoc-lib.sh`（这两档的 runner 需要 Rust），标签同样改为 `sqlite_fts5 anydoc`，否则发行包和 Task 1 的本地构建不一致。

链接若因缺 C++ 运行库或 Windows 系统库失败，把缺失的库补进 `anydoc.go` 的 Windows `LDFLAGS`。以 `windows-latest` 上这次 `go build` 的错误为准，不预先抄一份未验证的旗标。

Windows ARM 不加入矩阵。构建脚本对该目标保持失败。

- [ ] **Step 1:** 改 `build-anydoc-lib.sh` 的 gnu 目标与 msvc 输出目录
- [ ] **Step 2:** 在 `windows-latest` 上编出 `libanydoc_go.a`，并 `go build -tags "sqlite_fts5 anydoc" -o WeKnora-lite.exe ./cmd/server`
- [ ] **Step 3:** 用一份小 docx 走进程内 anydoc，解析结果里能看到预埋句子
- [ ] **Step 4:** `release-lite.yml` 的 `build-binary` 包含 `windows/amd64`，且每一档标签都是 `sqlite_fts5 anydoc`

---

## 验收

1. 只开问答通、本机 Ollama 和 `llm.json` 里的问答接口，不开文书通：在 `http://127.0.0.1:8090` 对已入库文档提问，得到非空回答和出处。对话请求只发往 `llm.json` 里的地址。该地址可以是本机、内网或互联网。
2. 该问答接口不可用时，页面问答失败，不改连另一个主机，也不改用 Ollama 做对话。
3. 只开文书通、不开问答通：排版与工作区对话仍可用；`corpus_health` 失败；回答不声称已查资料库。
4. 两个都开：txt、docx、pdf、html、xmind 的预埋句子能经文书通工具回到文书通的模型；工具走主进程，不走技能脚本，也不调用 `knowledge-chat`。
5. `8090` 与 `50051` 只出现在 `127.0.0.1` 上。
6. 文书通那把钥匙不能上传、不能删除、不能调用 `knowledge-chat`。
7. 问答接口在本机或内网时，切断互联网后，页面问答和 txt、文字版 pdf 的检索仍成功。把 `base_url` 改成 `https://api.openai.com/v1` 时，初始化不被地址类别拒绝。
8. Windows amd64：数据目录是 `%USERPROFILE%\.wenda-tong`。同一 Windows 用户下的文书通用 `Path.home()` 能读到 `config.json`。`icacls` 显示 `llm.json` 与 `config.json` 仅当前用户有权限。`docker port` 显示 docreader 在 `127.0.0.1:50051`。一份 docx 由链入的 anydoc 解析，不依赖 WSL。

## v2（本方案不实施）

本机视觉模型打开后，png/jpg/gif 与扫描版 PDF 的文字进入同一套检索。仍然只监听回环，仍然不调用云端 OCR。问答模型仍走 `llm.json`，地址可以是本机、内网或互联网。
