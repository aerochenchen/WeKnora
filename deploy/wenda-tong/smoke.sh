#!/bin/sh
# Upload the samples, wait until each one is parsed, then search for its sentence.
# Set SMOKE_CHAT=1 to also open a page session and read the knowledge-chat stream.
set -eu
cd "$(dirname "$0")"
exec python3 - "$@" <<'PY'
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

root = Path(".").resolve()
base = os.environ.get("WENDA_TONG_BASE", "http://127.0.0.1:8090").rstrip("/")
home = Path(os.environ.get("WENDA_TONG_HOME", Path.home() / ".wenda-tong"))
samples = root / "testdata"


def request(method, path, body=None, token="", content_type="application/json", timeout=60):
    data = body if isinstance(body, (bytes, bytearray)) or body is None else json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, method=method)
    if token:
        req.add_header("Authorization", "Bearer " + token)
    if content_type and body is not None:
        req.add_header("Content-Type", content_type)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read()


def login():
    account = json.loads((home / "account.json").read_text(encoding="utf-8"))
    status, raw = request(
        "POST",
        "/api/v1/auth/login",
        {"email": account["email"], "password": account["password"]},
    )
    if status != 200:
        raise SystemExit(f"login failed: {status} {raw[:300]!r}")
    payload = json.loads(raw)
    token = payload.get("token") or payload.get("data", {}).get("token")
    if not token:
        raise SystemExit("login response has no token")
    return token


def knowledge_base_id():
    cfg = json.loads((home / "config.json").read_text(encoding="utf-8"))
    kb = cfg.get("knowledge_base_id") or ""
    if not kb:
        raise SystemExit("config.json has no knowledge_base_id; run bootstrap first")
    return kb


def upload(token, kb, path: Path):
    boundary = "----wenda-tong-smoke"
    filename = path.name
    payload = path.read_bytes()
    body = b"".join(
        [
            f"--{boundary}\r\n".encode(),
            f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'.encode(),
            b"Content-Type: application/octet-stream\r\n\r\n",
            payload,
            b"\r\n",
            f"--{boundary}--\r\n".encode(),
        ]
    )
    return request(
        "POST",
        f"/api/v1/knowledge-bases/{kb}/knowledge/file",
        body,
        token,
        f"multipart/form-data; boundary={boundary}",
        timeout=120,
    )


def walk(value, found):
    if isinstance(value, dict):
        for item in value.values():
            walk(item, found)
    elif isinstance(value, list):
        for item in value:
            walk(item, found)
    elif isinstance(value, str):
        found.append(value)


def contains(raw: bytes, needle: str) -> bool:
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return needle in raw.decode("utf-8", errors="ignore")
    texts = []
    walk(payload, texts)
    return any(needle in text for text in texts)


def parse_status(token, knowledge_id):
    status, raw = request("GET", f"/api/v1/knowledge/{knowledge_id}", token=token)
    if status != 200:
        raise SystemExit(f"get knowledge {knowledge_id} failed: {status}")
    payload = json.loads(raw)
    data = payload.get("data") or payload
    return str(data.get("parse_status") or "")


def wait_parsed(token, knowledge_id):
    deadline = time.time() + 180
    last = ""
    while time.time() < deadline:
        last = parse_status(token, knowledge_id)
        if last in ("completed", "failed", "cancelled"):
            return last
        time.sleep(2)
    raise SystemExit(f"{knowledge_id} still {last or 'unknown'} after 180s")


def search(token, kb, sentence):
    return request(
        "POST",
        "/api/v1/knowledge-search",
        {
            "query": sentence,
            "knowledge_base_ids": [kb],
            "match_count": 5,
            "rerank": {"enabled": False},
        },
        token,
    )


def chat(token, kb, sentence):
    status, raw = request("POST", "/api/v1/sessions", {"title": "问答通冒烟"}, token)
    if status not in (200, 201):
        raise SystemExit(f"create session failed: {status} {raw[:300]!r}")
    payload = json.loads(raw)
    data = payload.get("data") or payload
    session_id = data.get("id") or data.get("session_id")
    if not session_id:
        raise SystemExit("session response has no id")
    status, raw = request(
        "POST",
        f"/api/v1/knowledge-chat/{session_id}",
        {
            "query": sentence,
            "knowledge_base_ids": [kb],
            "web_search_enabled": False,
        },
        token,
        timeout=180,
    )
    if status != 200 or not raw.strip():
        raise SystemExit(f"knowledge-chat failed: {status} {raw[:300]!r}")
    if sentence not in raw.decode("utf-8", errors="ignore") and "样例" not in raw.decode("utf-8", errors="ignore"):
        raise SystemExit("knowledge-chat answer did not mention the sample sentence or title")
    llm = json.loads((home / "llm.json").read_text(encoding="utf-8"))
    host = llm.get("base_url") or ""
    if not host.startswith("http://") and not host.startswith("https://"):
        raise SystemExit("llm.json base_url is not an http(s) address")
    print("chat ok; configured model host is", host)


def main():
    manifest = json.loads((samples / "manifest.json").read_text(encoding="utf-8"))
    token = login()
    kb = knowledge_base_id()
    for item in manifest:
        path = samples / item["file"]
        status, raw = upload(token, kb, path)
        expect = item["expect"]
        if expect == "rejected":
            if 200 <= status < 300:
                raise SystemExit(f"{item['file']} was accepted; audio and video must be refused")
            print(item["file"], "rejected", status)
            continue
        if status not in (200, 201):
            raise SystemExit(f"upload {item['file']} failed: {status} {raw[:300]!r}")
        payload = json.loads(raw)
        data = payload.get("data") or payload
        knowledge_id = data.get("id")
        if not knowledge_id:
            raise SystemExit(f"upload {item['file']} returned no id")
        parsed = wait_parsed(token, knowledge_id)
        if parsed != "completed":
            raise SystemExit(f"{item['file']} parse_status={parsed}")
        if expect == "status":
            print(item["file"], "completed")
            continue
        status, raw = search(token, kb, item["sentence"])
        if status != 200 or not contains(raw, item["sentence"]):
            raise SystemExit(f"search missed {item['file']}: {status} {raw[:300]!r}")
        print(item["file"], "found")
    if os.environ.get("SMOKE_CHAT") == "1":
        chat(token, kb, manifest[0]["sentence"])
    print("smoke ok")


if __name__ == "__main__":
    main()
PY
