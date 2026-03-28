# PicoClaw — Windows MCP Edition

Hardened, Windows-only fork of [sipeed/picoclaw](https://github.com/sipeed/picoclaw).

## Quick Start

```bat
make deps && make build
picoclaw.exe onboard
keytool.exe set claude sk-ant-YOUR-KEY
keytool.exe set openai sk-YOUR-KEY
keytool.exe set openrouter sk-or-YOUR-KEY
picoclaw.exe
```

## Security

- API keys: Windows Credential Manager (DPAPI) — never written to disk
- Gateway: 127.0.0.1 only, hardcoded
- MCP: localhost-only, enforced at startup
- File access: sandboxed to workspace
- Exec: allowlist only
- Single instance: named mutex

## Endpoints

| Endpoint | Description |
|---|---|
| GET  http://127.0.0.1:18790/health | Health |
| POST http://127.0.0.1:18790/chat   | Chat |
| POST http://127.0.0.1:18790/reset  | Reset |
| http://127.0.0.1:18800             | Web UI |
